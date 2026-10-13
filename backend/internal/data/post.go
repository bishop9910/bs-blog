package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/bs-blog/backend/internal/model"
	"github.com/bishop9910/bs-blog/backend/internal/pkg/textutil"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("data: not found")

// PostFilter describes one page of a post listing.
type PostFilter struct {
	// PublishedOnly restricts to posts visible on the public site: status
	// "published" AND visibility "public". It is set by the public endpoints
	// and never derived from user input.
	PublishedOnly bool
	// Status filters by draft/published; empty or "all" means no filter.
	Status string
	// TagSlug restricts to posts carrying a given tag.
	TagSlug string
	// Keyword is matched against title, summary and body.
	Keyword string
	// Sort is one of created, published, updated, title.
	Sort string
	// Order is asc or desc.
	Order    string
	Page     int
	PageSize int
}

// sortColumns whitelists the sortable fields. Mapping through a fixed table
// rather than interpolating the caller's string is what keeps ORDER BY from
// becoming an injection point.
var sortColumns = map[string]string{
	"created":   "created_at",
	"published": "published_at",
	"updated":   "updated_at",
	"title":     "title",
}

// Normalize clamps the filter into a safe, fully-populated state.
func (f *PostFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 10
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	if _, ok := sortColumns[f.Sort]; !ok {
		if f.PublishedOnly {
			f.Sort = "published"
		} else {
			f.Sort = "updated"
		}
	}
	if f.Order != "asc" && f.Order != "desc" {
		f.Order = "desc"
	}
	if f.Status == "all" {
		f.Status = ""
	}
}

// scoped builds the shared WHERE clause for a filter.
func (f PostFilter) scoped(ctx context.Context, db *gorm.DB) *gorm.DB {
	q := db.WithContext(ctx).Model(&model.Post{})

	if f.PublishedOnly {
		q = q.Where("status = ? AND visibility = ?", model.StatusPublished, model.VisibilityPublic)
	} else if f.Status == model.StatusDraft || f.Status == model.StatusPublished {
		q = q.Where("status = ?", f.Status)
	}

	if f.TagSlug != "" {
		q = q.Where("id IN (?)",
			db.WithContext(ctx).Table("post_tags").
				Select("post_tags.post_id").
				Joins("JOIN tags ON tags.id = post_tags.tag_id").
				Where("tags.slug = ?", f.TagSlug),
		)
	}

	if keyword := strings.TrimSpace(f.Keyword); keyword != "" {
		// Escape LIKE wildcards so a literal % does not match everything.
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
		like := "%" + escaped + "%"
		q = q.Where(
			`(title LIKE ? ESCAPE '\' OR summary LIKE ? ESCAPE '\' OR content_html LIKE ? ESCAPE '\')`,
			like, like, like,
		)
	}

	return q
}

// ListPosts returns one page of posts plus the total matching the filter.
func (s *Store) ListPosts(ctx context.Context, f PostFilter) ([]model.Post, int64, error) {
	f.Normalize()

	var total int64
	if err := f.scoped(ctx, s.db).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("data: counting posts: %w", err)
	}

	column := sortColumns[f.Sort]
	direction := strings.ToUpper(f.Order)

	// Drafts have a NULL published_at. SQLite sorts NULLs first ascending, so
	// order by nullness first to keep drafts consistently at one end.
	order := column + " " + direction + ", id DESC"
	if column == "published_at" {
		order = "published_at IS NULL, " + order
	}

	posts := make([]model.Post, 0)
	err := f.scoped(ctx, s.db).
		Preload("Tags").
		Order(order).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&posts).Error
	if err != nil {
		return nil, 0, fmt.Errorf("data: listing posts: %w", err)
	}
	return posts, total, nil
}

// GetPostByID loads one post with its tags.
func (s *Store) GetPostByID(ctx context.Context, id int64) (*model.Post, error) {
	var post model.Post
	err := s.db.WithContext(ctx).Preload("Tags").First(&post, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading post %d: %w", id, err)
	}
	return &post, nil
}

// GetPostBySlug loads one post by slug. When publishedOnly is set, drafts and
// private posts are reported as missing rather than as forbidden, so the public
// surface does not leak the existence of unpublished work.
func (s *Store) GetPostBySlug(ctx context.Context, slug string, publishedOnly bool) (*model.Post, error) {
	q := s.db.WithContext(ctx).Preload("Tags").Where("slug = ?", slug)
	if publishedOnly {
		q = q.Where("status = ? AND visibility = ?", model.StatusPublished, model.VisibilityPublic)
	}

	var post model.Post
	err := q.First(&post).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading post %q: %w", slug, err)
	}
	return &post, nil
}

// CreatePost inserts a post and links its tags in one transaction.
func (s *Store) CreatePost(ctx context.Context, post *model.Post, tagNames []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(post).Error; err != nil {
			return fmt.Errorf("data: creating post: %w", err)
		}
		tags, err := resolveTags(ctx, tx, tagNames)
		if err != nil {
			return err
		}
		if len(tags) > 0 {
			if err := tx.Model(post).Association("Tags").Replace(tags); err != nil {
				return fmt.Errorf("data: linking tags: %w", err)
			}
		}
		post.Tags = tags
		return nil
	})
}

// UpdatePost saves a post and replaces its tag set in one transaction.
func (s *Store) UpdatePost(ctx context.Context, post *model.Post, tagNames []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Select explicitly: Save on a struct with a zero-value field would
		// otherwise skip it, and clearing a summary or cover must be possible.
		if err := tx.Model(&model.Post{ID: post.ID}).Updates(map[string]any{
			"slug":             post.Slug,
			"title":            post.Title,
			"summary":          post.Summary,
			"cover_url":        post.CoverURL,
			"content_html":     post.ContentHTML,
			"content_json":     post.ContentJSON,
			"meta_title":       post.MetaTitle,
			"meta_description": post.MetaDescription,
			"visibility":       post.Visibility,
			"word_count":       post.WordCount,
			"reading_minutes":  post.ReadingMinutes,
		}).Error; err != nil {
			return fmt.Errorf("data: updating post %d: %w", post.ID, err)
		}

		tags, err := resolveTags(ctx, tx, tagNames)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.Post{ID: post.ID}).Association("Tags").Replace(tags); err != nil {
			return fmt.Errorf("data: linking tags: %w", err)
		}
		post.Tags = tags

		// Read back the authoritative timestamps rather than guessing them.
		return tx.First(post, post.ID).Error
	})
}

// DeletePost removes a post; the post_tags rows follow via cascade.
func (s *Store) DeletePost(ctx context.Context, id int64) error {
	res := s.db.WithContext(ctx).Delete(&model.Post{}, id)
	if res.Error != nil {
		return fmt.Errorf("data: deleting post %d: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPostStatus moves a post between draft and published, stamping
// PublishedAt on first publish and preserving it on later re-publishes.
func (s *Store) SetPostStatus(ctx context.Context, id int64, status string) (*model.Post, error) {
	var post model.Post
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&post, id).Error; err != nil {
			return err
		}

		updates := map[string]any{"status": status}
		if status == model.StatusPublished && post.PublishedAt == nil {
			now := time.Now().UTC()
			updates["published_at"] = now
			post.PublishedAt = &now
		}
		if err := tx.Model(&model.Post{ID: id}).Updates(updates).Error; err != nil {
			return err
		}
		post.Status = status
		return tx.Preload("Tags").First(&post, id).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: setting status of post %d: %w", id, err)
	}
	return &post, nil
}

// SlugTaken reports whether a slug is already used by a different post.
func (s *Store) SlugTaken(ctx context.Context, slug string, excludeID int64) (bool, error) {
	q := s.db.WithContext(ctx).Model(&model.Post{}).Where("slug = ?", slug)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, fmt.Errorf("data: checking slug %q: %w", slug, err)
	}
	return count > 0, nil
}

// IncrementView bumps a post's read counter.
func (s *Store) IncrementView(ctx context.Context, id int64) error {
	err := s.db.WithContext(ctx).Model(&model.Post{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	if err != nil {
		return fmt.Errorf("data: incrementing views of post %d: %w", id, err)
	}
	return nil
}

// PublishedPosts returns every publicly visible post, newest first. The archive
// view groups these in Go rather than in SQL, which keeps the grouping logic in
// one readable place.
func (s *Store) PublishedPosts(ctx context.Context) ([]model.Post, error) {
	posts := make([]model.Post, 0)
	err := s.db.WithContext(ctx).
		Where("status = ? AND visibility = ? AND published_at IS NOT NULL",
			model.StatusPublished, model.VisibilityPublic).
		Order("published_at DESC, id DESC").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("data: loading published posts: %w", err)
	}
	return posts, nil
}

// AdjacentPosts finds the neighbours of a post for previous/next navigation.
// "Previous" is the next-newest post older than this one; "next" is the
// next-oldest post newer than it. Either may be nil at the ends of the feed.
func (s *Store) AdjacentPosts(ctx context.Context, publishedAt time.Time, id int64) (prev, next *model.Post, err error) {
	base := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Where("status = ? AND visibility = ? AND published_at IS NOT NULL",
				model.StatusPublished, model.VisibilityPublic)
	}

	var older model.Post
	err = base().Where("(published_at < ?) OR (published_at = ? AND id < ?)", publishedAt, publishedAt, id).
		Order("published_at DESC, id DESC").First(&older).Error
	switch {
	case err == nil:
		prev = &older
	case errors.Is(err, gorm.ErrRecordNotFound):
		// No older post; prev stays nil.
	default:
		return nil, nil, fmt.Errorf("data: finding previous post: %w", err)
	}

	var newer model.Post
	err = base().Where("(published_at > ?) OR (published_at = ? AND id > ?)", publishedAt, publishedAt, id).
		Order("published_at ASC, id ASC").First(&newer).Error
	switch {
	case err == nil:
		next = &newer
	case errors.Is(err, gorm.ErrRecordNotFound):
		// No newer post; next stays nil.
	default:
		return nil, nil, fmt.Errorf("data: finding next post: %w", err)
	}

	return prev, next, nil
}

// Stats summarises the public archive for the site footer and about page.
type Stats struct {
	PostCount int64
	TagCount  int64
	WordCount int64
	First     *time.Time
	Last      *time.Time
}

// PublicStats aggregates only what an anonymous visitor can see.
func (s *Store) PublicStats(ctx context.Context) (Stats, error) {
	var stats Stats

	publicPosts := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&model.Post{}).
			Where("status = ? AND visibility = ?", model.StatusPublished, model.VisibilityPublic)
	}

	if err := publicPosts().Count(&stats.PostCount).Error; err != nil {
		return stats, fmt.Errorf("data: counting posts: %w", err)
	}
	if err := publicPosts().Select("COALESCE(SUM(word_count), 0)").Scan(&stats.WordCount).Error; err != nil {
		return stats, fmt.Errorf("data: summing words: %w", err)
	}

	// MIN/MAX over a time column come back from SQLite as strings, so take the
	// boundary rows instead and read their timestamps as proper time.Time.
	var first, last model.Post
	if err := publicPosts().Where("published_at IS NOT NULL").
		Order("published_at ASC").Limit(1).Find(&first).Error; err != nil {
		return stats, fmt.Errorf("data: finding first post: %w", err)
	}
	if err := publicPosts().Where("published_at IS NOT NULL").
		Order("published_at DESC").Limit(1).Find(&last).Error; err != nil {
		return stats, fmt.Errorf("data: finding last post: %w", err)
	}
	stats.First = first.PublishedAt
	stats.Last = last.PublishedAt

	err := s.db.WithContext(ctx).Model(&model.Tag{}).
		Where("id IN (?)",
			s.db.WithContext(ctx).Table("post_tags").Select("post_tags.tag_id").
				Joins("JOIN posts ON posts.id = post_tags.post_id").
				Where("posts.status = ? AND posts.visibility = ?", model.StatusPublished, model.VisibilityPublic),
		).
		Count(&stats.TagCount).Error
	if err != nil {
		return stats, fmt.Errorf("data: counting tags: %w", err)
	}

	return stats, nil
}

// resolveTags maps tag names onto rows, creating any that do not exist yet.
// Deduplication is by slug so "Vue" and "vue" collapse into one tag.
func resolveTags(ctx context.Context, tx *gorm.DB, names []string) ([]model.Tag, error) {
	tags := make([]model.Tag, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		slug := textutil.Slugify(name)
		if slug == "" {
			continue
		}
		if _, dup := seen[slug]; dup {
			continue
		}
		seen[slug] = struct{}{}

		var tag model.Tag
		err := tx.WithContext(ctx).Where("slug = ?", slug).First(&tag).Error
		switch {
		case err == nil:
			// Existing tag; reuse it.
		case errors.Is(err, gorm.ErrRecordNotFound):
			tag = model.Tag{Name: name, Slug: slug}
			if err := tx.WithContext(ctx).Create(&tag).Error; err != nil {
				return nil, fmt.Errorf("data: creating tag %q: %w", name, err)
			}
		default:
			return nil, fmt.Errorf("data: looking up tag %q: %w", name, err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}
