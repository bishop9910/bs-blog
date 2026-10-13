package biz

import (
	"time"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

/* -------------------------------------------------------------------------- */
/* Wire types                                                                 */
/*                                                                            */
/* These are the JSON contract with the frontend. Field names must match      */
/* src/types/blog.ts exactly, which is camelCase throughout.                  */
/* -------------------------------------------------------------------------- */

// TagDTO is a tag with its public post count.
type TagDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Color     string `json:"color,omitempty"`
	PostCount int64  `json:"postCount"`
}

// PostSummaryDTO is a post without its body — the shape used in lists.
type PostSummaryDTO struct {
	ID             int64    `json:"id"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	CoverURL       string   `json:"coverUrl"`
	Status         string   `json:"status"`
	Visibility     string   `json:"visibility"`
	Tags           []TagDTO `json:"tags"`
	PublishedAt    *string  `json:"publishedAt,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	ReadingMinutes int      `json:"readingMinutes"`
	WordCount      int      `json:"wordCount"`
	ViewCount      int64    `json:"viewCount"`
}

// PostDTO is a full post including its stored body.
type PostDTO struct {
	PostSummaryDTO
	ContentHTML     string `json:"contentHtml"`
	ContentJSON     string `json:"contentJson"`
	MetaTitle       string `json:"metaTitle,omitempty"`
	MetaDescription string `json:"metaDescription,omitempty"`
}

// PostLinkDTO is the minimal reference used by previous/next navigation.
type PostLinkDTO struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// PostNavDTO carries the neighbours of the post being read.
type PostNavDTO struct {
	Prev *PostLinkDTO `json:"prev,omitempty"`
	Next *PostLinkDTO `json:"next,omitempty"`
}

// PostDetailDTO is what the reader view fetches.
type PostDetailDTO struct {
	Post PostDTO    `json:"post"`
	Nav  PostNavDTO `json:"nav"`
}

// PaginatedPosts is one page of a post listing.
type PaginatedPosts struct {
	Items    []PostSummaryDTO `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

// ArchiveMonthDTO groups posts within one month.
type ArchiveMonthDTO struct {
	Month int              `json:"month"`
	Posts []PostSummaryDTO `json:"posts"`
}

// ArchiveYearDTO groups months within one year.
type ArchiveYearDTO struct {
	Year   int               `json:"year"`
	Count  int               `json:"count"`
	Months []ArchiveMonthDTO `json:"months"`
}

// ArchiveDTO wraps the year list.
type ArchiveDTO struct {
	Years []ArchiveYearDTO `json:"years"`
}

// TagListDTO wraps the public tag list.
type TagListDTO struct {
	Tags []TagDTO `json:"tags"`
}

// SocialLinkDTO is one entry in the site's social links.
type SocialLinkDTO struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// SiteStatsDTO is the aggregate shown in the footer.
type SiteStatsDTO struct {
	PostCount        int64   `json:"postCount"`
	TagCount         int64   `json:"tagCount"`
	WordCount        int64   `json:"wordCount"`
	FirstPublishedAt *string `json:"firstPublishedAt,omitempty"`
	LastPublishedAt  *string `json:"lastPublishedAt,omitempty"`
}

// SiteMetaDTO is everything the public shell needs to render.
type SiteMetaDTO struct {
	Title       string          `json:"title"`
	Tagline     string          `json:"tagline"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	AvatarURL   string          `json:"avatarUrl"`
	Socials     []SocialLinkDTO `json:"socials"`
	FooterNote  string          `json:"footerNote"`
	Stats       SiteStatsDTO    `json:"stats"`
}

// UserDTO is the signed-in administrator.
type UserDTO struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	AvatarURL   string  `json:"avatarUrl"`
	LastLoginAt *string `json:"lastLoginAt,omitempty"`
}

// LoginResultDTO is returned by both login and first-run setup.
type LoginResultDTO struct {
	Token     string  `json:"token"`
	ExpiresAt string  `json:"expiresAt"`
	User      UserDTO `json:"user"`
}

// MediaDTO is one uploaded file.
type MediaDTO struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CreatedAt string `json:"createdAt"`
}

// PaginatedMedia is one page of the media library.
type PaginatedMedia struct {
	Items    []MediaDTO `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// SlugCheckDTO answers the editor's slug field.
type SlugCheckDTO struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
}

/* -------------------------------------------------------------------------- */
/* Inputs                                                                     */
/* -------------------------------------------------------------------------- */

// PostInput is the body of a create or update request. Status is deliberately
// absent: publishing is an explicit, separate transition.
type PostInput struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Summary         string   `json:"summary"`
	CoverURL        string   `json:"coverUrl"`
	ContentHTML     string   `json:"contentHtml"`
	ContentJSON     string   `json:"contentJson"`
	TagNames        []string `json:"tagNames"`
	Visibility      string   `json:"visibility"`
	MetaTitle       string   `json:"metaTitle"`
	MetaDescription string   `json:"metaDescription"`
}

// Query is a parsed post listing request.
type Query struct {
	Page     int
	PageSize int
	Status   string
	Tag      string
	Keyword  string
	Sort     string
	Order    string
}

// SiteInput is the body of a site-profile update.
type SiteInput struct {
	Title       string          `json:"title"`
	Tagline     string          `json:"tagline"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	AvatarURL   string          `json:"avatarUrl"`
	Socials     []SocialLinkDTO `json:"socials"`
	FooterNote  string          `json:"footerNote"`
}

// ChangePasswordInput carries both the current and the replacement password.
type ChangePasswordInput struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// CredentialsInput is used by login and first-run setup.
type CredentialsInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

/* -------------------------------------------------------------------------- */
/* Mapping                                                                    */
/* -------------------------------------------------------------------------- */

// rfc3339 renders a timestamp in the format the frontend parses.
func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// rfc3339Ptr renders an optional timestamp.
func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := rfc3339(*t)
	return &s
}

func toTagDTO(tag model.Tag, postCount int64) TagDTO {
	return TagDTO{
		ID:        tag.ID,
		Name:      tag.Name,
		Slug:      tag.Slug,
		Color:     tag.Color,
		PostCount: postCount,
	}
}

func toTagsDTO(tags []model.Tag) []TagDTO {
	// Always a slice, never nil: the frontend maps over this directly.
	out := make([]TagDTO, 0, len(tags))
	for _, tag := range tags {
		out = append(out, toTagDTO(tag, 0))
	}
	return out
}

func toPostSummaryDTO(post model.Post) PostSummaryDTO {
	return PostSummaryDTO{
		ID:             post.ID,
		Slug:           post.Slug,
		Title:          post.Title,
		Summary:        post.Summary,
		CoverURL:       post.CoverURL,
		Status:         post.Status,
		Visibility:     post.Visibility,
		Tags:           toTagsDTO(post.Tags),
		PublishedAt:    rfc3339Ptr(post.PublishedAt),
		CreatedAt:      rfc3339(post.CreatedAt),
		UpdatedAt:      rfc3339(post.UpdatedAt),
		ReadingMinutes: post.ReadingMinutes,
		WordCount:      post.WordCount,
		ViewCount:      post.ViewCount,
	}
}

func toPostDTO(post model.Post) PostDTO {
	return PostDTO{
		PostSummaryDTO:  toPostSummaryDTO(post),
		ContentHTML:     post.ContentHTML,
		ContentJSON:     post.ContentJSON,
		MetaTitle:       post.MetaTitle,
		MetaDescription: post.MetaDescription,
	}
}

func toPostSummaries(posts []model.Post) []PostSummaryDTO {
	out := make([]PostSummaryDTO, 0, len(posts))
	for _, post := range posts {
		out = append(out, toPostSummaryDTO(post))
	}
	return out
}

func toPostLinkDTO(post *model.Post) *PostLinkDTO {
	if post == nil {
		return nil
	}
	return &PostLinkDTO{Slug: post.Slug, Title: post.Title}
}

func toUserDTO(user model.User) UserDTO {
	return UserDTO{
		ID:          user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		AvatarURL:   user.AvatarURL,
		LastLoginAt: rfc3339Ptr(user.LastLoginAt),
	}
}

func toMediaDTO(asset model.MediaAsset) MediaDTO {
	return MediaDTO{
		ID:        asset.ID,
		URL:       asset.URL,
		Filename:  asset.Filename,
		MimeType:  asset.MimeType,
		Size:      asset.Size,
		Width:     asset.Width,
		Height:    asset.Height,
		CreatedAt: rfc3339(asset.CreatedAt),
	}
}
