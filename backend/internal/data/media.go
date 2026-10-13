package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// CreateMedia records an uploaded file. The bytes are written to disk by the
// service layer; this row is the index of what exists.
func (s *Store) CreateMedia(ctx context.Context, asset *model.MediaAsset) error {
	if err := s.db.WithContext(ctx).Create(asset).Error; err != nil {
		return fmt.Errorf("data: creating media asset: %w", err)
	}
	return nil
}

// ListMedia pages through the library, newest first.
func (s *Store) ListMedia(ctx context.Context, page, pageSize int, keyword string) ([]model.MediaAsset, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}

	build := func() *gorm.DB {
		q := s.db.WithContext(ctx).Model(&model.MediaAsset{})
		if kw := strings.TrimSpace(keyword); kw != "" {
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(kw)
			q = q.Where(`filename LIKE ? ESCAPE '\'`, "%"+escaped+"%")
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("data: counting media: %w", err)
	}

	items := make([]model.MediaAsset, 0)
	err := build().
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("data: listing media: %w", err)
	}
	return items, total, nil
}

// GetMedia loads one asset by id.
func (s *Store) GetMedia(ctx context.Context, id int64) (*model.MediaAsset, error) {
	var asset model.MediaAsset
	err := s.db.WithContext(ctx).First(&asset, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading media %d: %w", id, err)
	}
	return &asset, nil
}

// DeleteMedia removes the index row and returns the asset that was deleted, so
// the caller can also unlink the file. Post references to the URL are left
// alone: a broken image in an old post is better than silently rewriting
// published content.
func (s *Store) DeleteMedia(ctx context.Context, id int64) (*model.MediaAsset, error) {
	asset, err := s.GetMedia(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Delete(&model.MediaAsset{}, id).Error; err != nil {
		return nil, fmt.Errorf("data: deleting media %d: %w", id, err)
	}
	return asset, nil
}
