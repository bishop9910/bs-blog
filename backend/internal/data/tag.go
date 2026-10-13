package data

import (
	"context"
	"fmt"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// TagWithCount is a tag plus how many publicly visible posts carry it.
type TagWithCount struct {
	model.Tag
	PostCount int64 `gorm:"column:post_count"`
}

// PublicTags lists tags that appear on at least one published, public post,
// most-used first. Tags whose every post is a draft are hidden, so the public
// tag cloud never advertises unwritten or withdrawn work.
func (s *Store) PublicTags(ctx context.Context) ([]TagWithCount, error) {
	rows := make([]TagWithCount, 0)
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, tags.slug, tags.color, tags.created_at, tags.updated_at, COUNT(post_tags.post_id) AS post_count").
		Joins("JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("JOIN posts ON posts.id = post_tags.post_id AND posts.status = ? AND posts.visibility = ?",
			model.StatusPublished, model.VisibilityPublic).
		Group("tags.id").
		Order("post_count DESC, tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("data: listing public tags: %w", err)
	}
	return rows, nil
}

// AllTags lists every tag with its total post count, drafts included. Used by
// the admin tag picker.
func (s *Store) AllTags(ctx context.Context) ([]TagWithCount, error) {
	rows := make([]TagWithCount, 0)
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, tags.slug, tags.color, tags.created_at, tags.updated_at, COUNT(post_tags.post_id) AS post_count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Group("tags.id").
		Order("post_count DESC, tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("data: listing tags: %w", err)
	}
	return rows, nil
}

// GetTagBySlug loads a single tag.
func (s *Store) GetTagBySlug(ctx context.Context, slug string) (*model.Tag, error) {
	var tag model.Tag
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&tag).Error; err != nil {
		return nil, fmt.Errorf("data: loading tag %q: %w", slug, err)
	}
	return &tag, nil
}
