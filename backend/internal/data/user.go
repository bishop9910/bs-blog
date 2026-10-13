package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// CountUsers reports how many accounts exist. A count of zero is what makes
// the setup/claim endpoint available.
func (s *Store) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("data: counting users: %w", err)
	}
	return count, nil
}

// GetUserByUsername loads an account for login.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading user %q: %w", username, err)
	}
	return &user, nil
}

// GetUserByID loads an account by primary key.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("data: loading user %d: %w", id, err)
	}
	return &user, nil
}

// CreateUser inserts an account. Username uniqueness is enforced by the index,
// so a race between two setup attempts fails loudly instead of creating a
// second owner.
func (s *Store) CreateUser(ctx context.Context, user *model.User) error {
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("data: creating user: %w", err)
	}
	return nil
}

// UpdateUserProfile changes the display fields of an account.
func (s *Store) UpdateUserProfile(ctx context.Context, id int64, nickname, avatarURL string) error {
	err := s.db.WithContext(ctx).Model(&model.User{ID: id}).Updates(map[string]any{
		"nickname":   nickname,
		"avatar_url": avatarURL,
	}).Error
	if err != nil {
		return fmt.Errorf("data: updating profile of user %d: %w", id, err)
	}
	return nil
}

// UpdatePasswordAndRevokeTokens replaces the stored hash and increments the
// user's token version in one transaction.
//
// Doing both together is the point: the hash and the version must never
// disagree, or a password change would either leave old sessions alive or lock
// the owner out of the new one.
func (s *Store) UpdatePasswordAndRevokeTokens(ctx context.Context, id int64, passwordHash string) error {
	err := s.db.WithContext(ctx).Model(&model.User{ID: id}).Updates(map[string]any{
		"password_hash": passwordHash,
		"token_version": gorm.Expr("token_version + 1"),
	}).Error
	if err != nil {
		return fmt.Errorf("data: updating password of user %d: %w", id, err)
	}
	return nil
}

// TouchLastLogin records a successful sign-in.
func (s *Store) TouchLastLogin(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Model(&model.User{ID: id}).
		Update("last_login_at", now).Error
	if err != nil {
		return fmt.Errorf("data: recording login of user %d: %w", id, err)
	}
	return nil
}
