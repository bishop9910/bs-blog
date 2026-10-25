package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// CreateResource records one uploaded file. The bytes are written to disk by the
// use case; this row is the index of what exists.
func (s *Store) CreateResource(ctx context.Context, file *model.ResourceFile) error {
	if err := s.db.WithContext(ctx).Create(file).Error; err != nil {
		return fmt.Errorf("data: creating resource: %w", err)
	}
	return nil
}

// ListResources pages through the collection, newest first.
//
// visibleOnly is what separates the public page from the admin list: a file the
// owner hid must not appear to a visitor, while the owner still has to find it
// to unhide it.
func (s *Store) ListResources(
	ctx context.Context,
	page, pageSize int,
	keyword string,
	visibleOnly bool,
) ([]model.ResourceFile, int64, error) {
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
		q := s.db.WithContext(ctx).Model(&model.ResourceFile{})
		if visibleOnly {
			q = q.Where("visible = ?", true)
		}
		if kw := strings.TrimSpace(keyword); kw != "" {
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(kw)
			pattern := "%" + escaped + "%"
			// Title is what the visitor searches by, filename is what they may
			// remember it as. One box should match either.
			q = q.Where(`title LIKE ? ESCAPE '\' OR filename LIKE ? ESCAPE '\'`, pattern, pattern)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("data: counting resources: %w", err)
	}

	items := make([]model.ResourceFile, 0)
	err := build().
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("data: listing resources: %w", err)
	}
	return items, total, nil
}

// GetResource loads one file row by id.
func (s *Store) GetResource(ctx context.Context, id int64) (*model.ResourceFile, error) {
	var file model.ResourceFile
	err := s.db.WithContext(ctx).First(&file, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading resource %d: %w", id, err)
	}
	return &file, nil
}

// UpdateResource writes the mutable metadata back.
//
// Visible is included, so hiding and unhiding take the same path as any other
// edit. The file on disk and its StoredName are never touched: replacing the
// bytes under an existing id would make a published download link silently
// serve something else.
func (s *Store) UpdateResource(ctx context.Context, file *model.ResourceFile) error {
	err := s.db.WithContext(ctx).
		Model(&model.ResourceFile{}).
		Where("id = ?", file.ID).
		Updates(map[string]any{
			"title":       file.Title,
			"description": file.Description,
			"cover_url":   file.CoverURL,
			"visible":     file.Visible,
		}).Error
	if err != nil {
		return fmt.Errorf("data: updating resource %d: %w", file.ID, err)
	}
	return nil
}

// DeleteResource removes the index row and returns the file that was deleted, so
// the caller can unlink the bytes too.
func (s *Store) DeleteResource(ctx context.Context, id int64) (*model.ResourceFile, error) {
	file, err := s.GetResource(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Delete(&model.ResourceFile{}, id).Error; err != nil {
		return nil, fmt.Errorf("data: deleting resource %d: %w", id, err)
	}
	return file, nil
}

// IncrementResourceDownloads counts one download.
//
// A download is counted when the bytes are about to be written, not when they
// have all left the process: the response is already committed by then, and
// waiting would let an aborted transfer throw the count away. The number is
// vanity, so an approximate count is the right trade.
func (s *Store) IncrementResourceDownloads(ctx context.Context, id int64) error {
	err := s.db.WithContext(ctx).
		Model(&model.ResourceFile{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
	if err != nil {
		return fmt.Errorf("data: counting download %d: %w", id, err)
	}
	return nil
}
