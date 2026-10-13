// Package model holds the GORM entities — the persistence shape of the blog.
//
// These are deliberately separate from the JSON DTOs in internal/service: the
// wire format is camelCase and the DB is snake_case, and keeping them apart
// means renaming a column never silently changes the public API.
package model

import "time"

// Post status values.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Post visibility values.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// User is the blog owner. This app has exactly one, created on first run.
type User struct {
	ID       int64  `gorm:"primaryKey"`
	Username string `gorm:"size:64;uniqueIndex;not null"`
	Nickname string `gorm:"size:64;not null;default:''"`
	// PasswordHash is a bcrypt digest; never leaves the process.
	PasswordHash string `gorm:"size:100;not null"`
	// TokenVersion is bumped whenever the password changes. It is copied into
	// every issued token, so changing the password immediately invalidates all
	// previously issued tokens — a stateless JWT cannot be recalled otherwise.
	TokenVersion int64      `gorm:"not null;default:0"`
	AvatarURL    string     `gorm:"size:512;not null;default:''"`
	LastLoginAt  *time.Time `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName keeps the entity name out of the table name.
func (User) TableName() string { return "users" }

// Post is a single article.
type Post struct {
	ID      int64  `gorm:"primaryKey"`
	Slug    string `gorm:"size:160;uniqueIndex;not null"`
	Title   string `gorm:"size:255;not null"`
	Summary string `gorm:"type:text;not null;default:''"`
	// CoverURL is either an /uploads/... path or an absolute URL.
	CoverURL string `gorm:"size:512;not null;default:''"`

	// ContentHTML is the sanitised canonical body. ContentJSON is the
	// ProseMirror document, kept so the editor can round-trip losslessly.
	ContentHTML string `gorm:"type:text;not null;default:''"`
	ContentJSON string `gorm:"type:text;not null;default:''"`

	MetaTitle       string `gorm:"size:255;not null;default:''"`
	MetaDescription string `gorm:"size:512;not null;default:''"`

	Status     string `gorm:"size:16;not null;default:'draft';index"`
	Visibility string `gorm:"size:16;not null;default:'public';index"`

	// Derived from the body so listings never have to parse HTML.
	WordCount      int   `gorm:"not null;default:0"`
	ReadingMinutes int   `gorm:"not null;default:1"`
	ViewCount      int64 `gorm:"not null;default:0"`

	// PublishedAt is set on first publish and kept afterwards, so unpublishing
	// and republishing preserves the original date.
	PublishedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"index"`
	UpdatedAt   time.Time

	Tags []Tag `gorm:"many2many:post_tags;constraint:OnDelete:CASCADE"`
}

// TableName avoids the reserved word "posts" colliding with anything else.
func (Post) TableName() string { return "posts" }

// Tag is a free-form label; posts and tags are many-to-many.
type Tag struct {
	ID    int64  `gorm:"primaryKey"`
	Name  string `gorm:"size:64;uniqueIndex;not null"`
	Slug  string `gorm:"size:80;uniqueIndex;not null"`
	Color string `gorm:"size:16;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName is explicit for symmetry with the other entities.
func (Tag) TableName() string { return "tags" }

// MediaAsset records one uploaded image.
type MediaAsset struct {
	ID int64 `gorm:"primaryKey"`
	// URL is what the browser requests, e.g. /uploads/2026/04/ab12cd34.png.
	URL string `gorm:"size:512;not null"`
	// Filename is the name the user's file had, kept for display and search.
	Filename string `gorm:"size:255;not null"`
	// StoredName is the on-disk basename, relative to the upload directory.
	StoredName string `gorm:"size:255;not null"`
	MimeType   string `gorm:"size:128;not null"`
	Size       int64  `gorm:"not null;default:0"`
	Width      int    `gorm:"not null;default:0"`
	Height     int    `gorm:"not null;default:0"`

	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
}

// TableName keeps the entity name out of the table name.
func (MediaAsset) TableName() string { return "media_assets" }

// SiteProfile is the single row of site-wide metadata (id = 1).
type SiteProfile struct {
	ID          int64  `gorm:"primaryKey"`
	Title       string `gorm:"size:120;not null;default:''"`
	Tagline     string `gorm:"size:200;not null;default:''"`
	Description string `gorm:"size:500;not null;default:''"`
	Author      string `gorm:"size:80;not null;default:''"`
	AvatarURL   string `gorm:"size:512;not null;default:''"`
	// Socials is a JSON array of {icon,label,url} objects.
	Socials    string `gorm:"type:text;not null;default:'[]'"`
	FooterNote string `gorm:"size:300;not null;default:''"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName keeps the entity name out of the table name.
func (SiteProfile) TableName() string { return "site_profile" }

// All returns every entity, for AutoMigrate.
func All() []any {
	return []any{
		&User{},
		&Post{},
		&Tag{},
		&MediaAsset{},
		&SiteProfile{},
	}
}
