package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// DefaultSocials is the empty social link list, stored as a JSON string.
const DefaultSocials = "[]"

// DefaultWallpapers is the empty wallpaper pool, stored as a JSON string.
const DefaultWallpapers = "[]"

// GetSiteProfile returns the singleton site profile, creating a sensible
// default row the first time it is asked for. A personal blog with one owner
// does not need a separate "has the site been configured" flag; the row's
// existence is that flag.
func (s *Store) GetSiteProfile(ctx context.Context) (*model.SiteProfile, error) {
	var profile model.SiteProfile
	err := s.db.WithContext(ctx).First(&profile, 1).Error
	if err == nil {
		return &profile, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("data: loading site profile: %w", err)
	}

	profile = model.SiteProfile{
		ID:          1,
		Title:       "bs-blog",
		Tagline:     "写点东西",
		Description: "一个人的写作空间。",
		Author:      "bs",
		Wallpapers:  DefaultWallpapers,
		Socials:     DefaultSocials,
	}
	if err := s.db.WithContext(ctx).Create(&profile).Error; err != nil {
		// A concurrent request may have created the row first; re-read rather
		// than failing, since the outcome the caller wants already exists.
		var existing model.SiteProfile
		if readErr := s.db.WithContext(ctx).First(&existing, 1).Error; readErr == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("data: creating site profile: %w", err)
	}
	return &profile, nil
}

// SaveSiteProfile persists edits to the site profile.
func (s *Store) SaveSiteProfile(ctx context.Context, profile *model.SiteProfile) error {
	if profile.ID == 0 {
		profile.ID = 1
	}
	if profile.Socials == "" {
		profile.Socials = DefaultSocials
	}
	if profile.Wallpapers == "" {
		profile.Wallpapers = DefaultWallpapers
	}
	// Validate here so a malformed payload cannot poison every later read that
	// tries to unmarshal it.
	if !json.Valid([]byte(profile.Socials)) {
		return fmt.Errorf("data: socials is not valid JSON")
	}
	if !json.Valid([]byte(profile.Wallpapers)) {
		return fmt.Errorf("data: wallpapers is not valid JSON")
	}

	err := s.db.WithContext(ctx).Model(&model.SiteProfile{ID: profile.ID}).Updates(map[string]any{
		"title":       profile.Title,
		"tagline":     profile.Tagline,
		"description": profile.Description,
		"author":      profile.Author,
		"avatar_url":  profile.AvatarURL,
		"wallpapers":  profile.Wallpapers,
		"socials":     profile.Socials,
		"footer_note": profile.FooterNote,
	}).Error
	if err != nil {
		return fmt.Errorf("data: saving site profile: %w", err)
	}
	return nil
}
