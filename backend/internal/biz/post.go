package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/model"
	"github.com/bishop9910/bs-blog/backend/internal/pkg/sanitize"
	"github.com/bishop9910/bs-blog/backend/internal/pkg/textutil"
)

// Limits on user-supplied post fields.
const (
	maxTitleLength  = 255
	maxSlugLength   = 80
	maxSummaryRunes = 200
	defaultExcerpt  = 160
	emptyDocJSON    = "{}"
)

// PostUsecase holds the rules for reading and writing posts.
type PostUsecase struct {
	store *data.Store
}

// NewPostUsecase wires the use case to storage.
func NewPostUsecase(store *data.Store) *PostUsecase {
	return &PostUsecase{store: store}
}

/* -------------------------------------------------------------------------- */
/* Reads                                                                      */
/* -------------------------------------------------------------------------- */

// ListPublic returns one page of published, public posts.
func (uc *PostUsecase) ListPublic(ctx context.Context, q Query) (*PaginatedPosts, error) {
	return uc.list(ctx, q, true)
}

// ListAdmin returns one page of posts of any status.
func (uc *PostUsecase) ListAdmin(ctx context.Context, q Query) (*PaginatedPosts, error) {
	return uc.list(ctx, q, false)
}

func (uc *PostUsecase) list(ctx context.Context, q Query, public bool) (*PaginatedPosts, error) {
	filter := data.PostFilter{
		PublishedOnly: public,
		Status:        strings.TrimSpace(q.Status),
		TagSlug:       strings.TrimSpace(q.Tag),
		Keyword:       strings.TrimSpace(q.Keyword),
		Sort:          strings.TrimSpace(q.Sort),
		Order:         strings.TrimSpace(q.Order),
		Page:          q.Page,
		PageSize:      q.PageSize,
	}
	filter.Normalize()

	posts, total, err := uc.store.ListPosts(ctx, filter)
	if err != nil {
		return nil, internal("无法加载文章列表", err)
	}

	return &PaginatedPosts{
		Items:    toPostSummaries(posts),
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// GetPublished returns one article for the reader view, with its neighbours.
func (uc *PostUsecase) GetPublished(ctx context.Context, slug string) (*PostDetailDTO, error) {
	post, err := uc.store.GetPostBySlug(ctx, strings.TrimSpace(slug), true)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("文章不存在或尚未发布")
		}
		return nil, internal("无法加载文章", err)
	}

	// Counting the read is best-effort: a failure must not cost the reader the
	// article they asked for.
	if err := uc.store.IncrementView(ctx, post.ID); err != nil {
		log.Errorf("biz: incrementing view count for post %d: %v", post.ID, err)
	} else {
		post.ViewCount++
	}

	nav := PostNavDTO{}
	if post.PublishedAt != nil {
		prev, next, err := uc.store.AdjacentPosts(ctx, *post.PublishedAt, post.ID)
		if err != nil {
			// Navigation is a nicety; losing it is not worth an error page.
			log.Errorf("biz: loading neighbours of post %d: %v", post.ID, err)
		} else {
			nav.Prev = toPostLinkDTO(prev)
			nav.Next = toPostLinkDTO(next)
		}
	}

	return &PostDetailDTO{Post: toPostDTO(*post), Nav: nav}, nil
}

// GetAdmin returns any post by id, drafts included.
func (uc *PostUsecase) GetAdmin(ctx context.Context, id int64) (*PostDTO, error) {
	post, err := uc.store.GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("文章不存在")
		}
		return nil, internal("无法加载文章", err)
	}
	dto := toPostDTO(*post)
	return &dto, nil
}

// Archive groups published posts by year and month, newest first.
func (uc *PostUsecase) Archive(ctx context.Context) (*ArchiveDTO, error) {
	posts, err := uc.store.PublishedPosts(ctx)
	if err != nil {
		return nil, internal("无法加载归档", err)
	}

	// Posts arrive newest-first, so appending preserves that order in the
	// year and month buckets as well.
	years := make([]ArchiveYearDTO, 0, 4)
	yearIndex := make(map[int]int, 4)
	monthIndex := make(map[[2]int]int, 12)

	for _, post := range posts {
		if post.PublishedAt == nil {
			continue
		}
		published := post.PublishedAt.UTC()
		year, month := published.Year(), int(published.Month())

		yi, ok := yearIndex[year]
		if !ok {
			years = append(years, ArchiveYearDTO{Year: year, Months: []ArchiveMonthDTO{}})
			yi = len(years) - 1
			yearIndex[year] = yi
		}

		key := [2]int{year, month}
		mi, ok := monthIndex[key]
		if !ok {
			years[yi].Months = append(years[yi].Months, ArchiveMonthDTO{Month: month, Posts: []PostSummaryDTO{}})
			mi = len(years[yi].Months) - 1
			monthIndex[key] = mi
		}

		years[yi].Months[mi].Posts = append(years[yi].Months[mi].Posts, toPostSummaryDTO(post))
		years[yi].Count++
	}

	return &ArchiveDTO{Years: years}, nil
}

// Tags lists the tags that appear on at least one public post.
func (uc *PostUsecase) Tags(ctx context.Context) (*TagListDTO, error) {
	rows, err := uc.store.PublicTags(ctx)
	if err != nil {
		return nil, internal("无法加载标签", err)
	}
	out := make([]TagDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTagDTO(row.Tag, row.PostCount))
	}
	return &TagListDTO{Tags: out}, nil
}

// CheckSlug answers whether a slug is free, and echoes back the normalised form
// so the editor can show the user what will actually be stored.
func (uc *PostUsecase) CheckSlug(ctx context.Context, raw string, excludeID int64) (*SlugCheckDTO, error) {
	slug := textutil.Slugify(strings.TrimSpace(raw))
	if slug == "" {
		return &SlugCheckDTO{Slug: "", Available: false}, nil
	}
	taken, err := uc.store.SlugTaken(ctx, slug, excludeID)
	if err != nil {
		return nil, internal("无法检查 slug", err)
	}
	return &SlugCheckDTO{Slug: slug, Available: !taken}, nil
}

/* -------------------------------------------------------------------------- */
/* Writes                                                                     */
/* -------------------------------------------------------------------------- */

// Create stores a new post. It always starts as a draft; publishing is a
// separate transition so that saving a half-finished thought is never
// accidentally visible.
func (uc *PostUsecase) Create(ctx context.Context, in PostInput) (*PostDTO, error) {
	title, err := validateTitle(in.Title)
	if err != nil {
		return nil, err
	}

	slug, err := uc.resolveSlug(ctx, in.Slug, title, 0)
	if err != nil {
		return nil, err
	}

	contentHTML := sanitize.HTML(in.ContentHTML)
	contentJSON := strings.TrimSpace(in.ContentJSON)
	if contentJSON == "" {
		contentJSON = emptyDocJSON
	}

	post := &model.Post{
		Slug:            slug,
		Title:           title,
		Summary:         resolveSummary(in.Summary, contentHTML),
		CoverURL:        strings.TrimSpace(in.CoverURL),
		ContentHTML:     contentHTML,
		ContentJSON:     contentJSON,
		MetaTitle:       strings.TrimSpace(in.MetaTitle),
		MetaDescription: strings.TrimSpace(in.MetaDescription),
		Status:          model.StatusDraft,
		Visibility:      normalizeVisibility(in.Visibility),
		WordCount:       textutil.CountWords(contentHTML),
		ReadingMinutes:  textutil.ReadingMinutes(contentHTML),
	}

	if err := uc.store.CreatePost(ctx, post, in.TagNames); err != nil {
		return nil, internal("无法保存文章", err)
	}

	dto := toPostDTO(*post)
	return &dto, nil
}

// Update overwrites a post's editable fields. Status and publishedAt are not
// touched here — see Publish and Unpublish.
func (uc *PostUsecase) Update(ctx context.Context, id int64, in PostInput) (*PostDTO, error) {
	post, err := uc.store.GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("文章不存在")
		}
		return nil, internal("无法加载文章", err)
	}

	title, err := validateTitle(in.Title)
	if err != nil {
		return nil, err
	}

	slug, err := uc.resolveSlug(ctx, in.Slug, title, id)
	if err != nil {
		return nil, err
	}

	contentHTML := sanitize.HTML(in.ContentHTML)
	contentJSON := strings.TrimSpace(in.ContentJSON)
	if contentJSON == "" {
		contentJSON = emptyDocJSON
	}

	post.Slug = slug
	post.Title = title
	post.Summary = resolveSummary(in.Summary, contentHTML)
	post.CoverURL = strings.TrimSpace(in.CoverURL)
	post.ContentHTML = contentHTML
	post.ContentJSON = contentJSON
	post.MetaTitle = strings.TrimSpace(in.MetaTitle)
	post.MetaDescription = strings.TrimSpace(in.MetaDescription)
	post.Visibility = normalizeVisibility(in.Visibility)
	post.WordCount = textutil.CountWords(contentHTML)
	post.ReadingMinutes = textutil.ReadingMinutes(contentHTML)

	if err := uc.store.UpdatePost(ctx, post, in.TagNames); err != nil {
		return nil, internal("无法保存文章", err)
	}

	dto := toPostDTO(*post)
	return &dto, nil
}

// Delete removes a post and its tag links.
func (uc *PostUsecase) Delete(ctx context.Context, id int64) error {
	if err := uc.store.DeletePost(ctx, id); err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return errNotFoundError("文章不存在")
		}
		return internal("无法删除文章", err)
	}
	return nil
}

// Publish moves a post to the public surface, stamping publishedAt the first
// time only.
func (uc *PostUsecase) Publish(ctx context.Context, id int64) (*PostDTO, error) {
	return uc.setStatus(ctx, id, model.StatusPublished)
}

// Unpublish returns a post to draft, keeping its original publishedAt so a
// later re-publish does not reorder the archive.
func (uc *PostUsecase) Unpublish(ctx context.Context, id int64) (*PostDTO, error) {
	return uc.setStatus(ctx, id, model.StatusDraft)
}

func (uc *PostUsecase) setStatus(ctx context.Context, id int64, status string) (*PostDTO, error) {
	post, err := uc.store.SetPostStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, data.ErrNotFound) {
			return nil, errNotFoundError("文章不存在")
		}
		return nil, internal("无法更新文章状态", err)
	}
	dto := toPostDTO(*post)
	return &dto, nil
}

/* -------------------------------------------------------------------------- */
/* Helpers                                                                    */
/* -------------------------------------------------------------------------- */

func validateTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" {
		return "", errBadRequest("标题不能为空")
	}
	if len([]rune(title)) > maxTitleLength {
		return "", errBadRequest("标题过长")
	}
	return title, nil
}

// resolveSlug decides the stored slug.
//
// A slug the author typed is honoured, and a collision is reported rather than
// silently renamed — the editor already checks availability, so a conflict here
// means something changed underneath them. A slug derived from the title is
// allowed to auto-suffix, because the author never asked for that exact string.
func (uc *PostUsecase) resolveSlug(ctx context.Context, requested, title string, excludeID int64) (string, error) {
	explicit := strings.TrimSpace(requested)
	if explicit != "" {
		slug := truncateRunes(textutil.Slugify(explicit), maxSlugLength)
		if slug == "" {
			return "", errBadRequest("slug 只能包含字母、数字或汉字")
		}
		taken, err := uc.store.SlugTaken(ctx, slug, excludeID)
		if err != nil {
			return "", internal("无法检查 slug", err)
		}
		if taken {
			return "", errConflict("该 slug 已被其他文章占用")
		}
		return slug, nil
	}

	base := truncateRunes(textutil.Slugify(title), maxSlugLength)
	if base == "" {
		base = fmt.Sprintf("post-%d", time.Now().UTC().Unix())
	}

	candidate := base
	for attempt := 2; attempt <= 100; attempt++ {
		taken, err := uc.store.SlugTaken(ctx, candidate, excludeID)
		if err != nil {
			return "", internal("无法检查 slug", err)
		}
		if !taken {
			return candidate, nil
		}
		candidate = truncateRunes(fmt.Sprintf("%s-%d", base, attempt), maxSlugLength)
	}

	return "", errConflict("无法为该标题生成唯一 slug，请手动指定")
}

// resolveSummary prefers an author-written summary and otherwise derives one
// from the body.
func resolveSummary(explicit, contentHTML string) string {
	if s := strings.TrimSpace(explicit); s != "" {
		return truncateRunes(s, maxSummaryRunes)
	}
	return textutil.AutoExcerpt(contentHTML, defaultExcerpt)
}

func normalizeVisibility(raw string) string {
	if strings.TrimSpace(raw) == model.VisibilityPrivate {
		return model.VisibilityPrivate
	}
	return model.VisibilityPublic
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return strings.Trim(string(runes[:limit]), "-")
}
