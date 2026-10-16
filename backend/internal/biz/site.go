package biz

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// SiteLimits bounds everything the settings page may store.
//
// Two of these come from config.yaml (site.maxWallpapers and
// site.maxWallpaperLength) because they are worth tuning per deployment. The
// rest are matched to the column sizes declared in internal/model and stay code
// constants — but they are still reported through GET /admin/limits, so the
// settings form can pre-check without duplicating the numbers.
//
// The values live on SiteLimitsDTO so the settings page reads back exactly what
// is enforced here.

// DefaultSiteLimits is the schema-matched field policy.
func DefaultSiteLimits() SiteLimitsDTO {
	return SiteLimitsDTO{
		MaxSocials:           12,
		MaxWallpapers:        24,
		MaxWallpaperLength:   512,
		MaxTitleLength:       60,
		MaxTaglineLength:     100,
		MaxDescriptionLength: 300,
		MaxAuthorLength:      40,
		MaxFooterLength:      120,
		MaxSocialLabelLength: 40,
	}
}

// knownSocialIcons keeps the frontend's icon lookup total. An unrecognised key
// is downgraded rather than dropped, so a link never disappears from the page
// just because its icon name was unexpected.
var knownSocialIcons = map[string]struct{}{
	"github": {},
	"x":      {},
	"email":  {},
	"rss":    {},
	"link":   {},
}

// SiteUsecase reads and writes the site's singleton metadata.
type SiteUsecase struct {
	store  *data.Store
	limits SiteLimitsDTO
}

// NewSiteUsecase wires the use case. Limits that are not set fall back to the
// defaults, so a zero value cannot silently disable a field.
func NewSiteUsecase(store *data.Store, limits SiteLimitsDTO) *SiteUsecase {
	defaults := DefaultSiteLimits()
	if limits.MaxSocials <= 0 {
		limits.MaxSocials = defaults.MaxSocials
	}
	if limits.MaxWallpaperLength <= 0 {
		limits.MaxWallpaperLength = defaults.MaxWallpaperLength
	}
	if limits.MaxTitleLength <= 0 {
		limits.MaxTitleLength = defaults.MaxTitleLength
	}
	if limits.MaxTaglineLength <= 0 {
		limits.MaxTaglineLength = defaults.MaxTaglineLength
	}
	if limits.MaxDescriptionLength <= 0 {
		limits.MaxDescriptionLength = defaults.MaxDescriptionLength
	}
	if limits.MaxAuthorLength <= 0 {
		limits.MaxAuthorLength = defaults.MaxAuthorLength
	}
	if limits.MaxFooterLength <= 0 {
		limits.MaxFooterLength = defaults.MaxFooterLength
	}
	if limits.MaxSocialLabelLength <= 0 {
		limits.MaxSocialLabelLength = defaults.MaxSocialLabelLength
	}
	// MaxWallpapers is deliberately left alone: 0 is a meaningful setting that
	// turns the wallpaper pool off.
	return &SiteUsecase{store: store, limits: limits}
}

// Limits reports the effective field policy for the settings page.
func (uc *SiteUsecase) Limits() SiteLimitsDTO { return uc.limits }

// Get returns the public site metadata, including aggregate stats.
func (uc *SiteUsecase) Get(ctx context.Context) (*SiteMetaDTO, error) {
	profile, err := uc.store.GetSiteProfile(ctx)
	if err != nil {
		return nil, internal("无法读取站点信息", err)
	}

	stats, err := uc.store.PublicStats(ctx)
	if err != nil {
		return nil, internal("无法读取站点统计", err)
	}

	return toSiteMetaDTO(*profile, stats), nil
}

// Update persists edits to the site metadata and returns the stored result.
//
// The response is re-read rather than echoed back, so the client always sees
// exactly what a fresh visitor would see.
func (uc *SiteUsecase) Update(ctx context.Context, in SiteInput) (*SiteMetaDTO, error) {
	profile, err := uc.store.GetSiteProfile(ctx)
	if err != nil {
		return nil, internal("无法读取站点信息", err)
	}

	// An empty title falls back to the stored one rather than failing: the
	// frontend's update payload is typed as Partial<SiteMeta>, so a field the
	// caller did not include arrives empty and must not read as "clear it".
	title := truncateRunes(strings.TrimSpace(in.Title), uc.limits.MaxTitleLength)
	if title == "" {
		title = profile.Title
	}

	socials, err := json.Marshal(uc.normaliseSocials(in.Socials))
	if err != nil {
		return nil, internal("无法保存社交链接", err)
	}

	wallpapers, err := json.Marshal(uc.normaliseWallpapers(in.Wallpapers))
	if err != nil {
		return nil, internal("无法保存壁纸列表", err)
	}

	profile.Title = title
	profile.Tagline = truncateRunes(strings.TrimSpace(in.Tagline), uc.limits.MaxTaglineLength)
	profile.Description = truncateRunes(strings.TrimSpace(in.Description), uc.limits.MaxDescriptionLength)
	profile.Author = truncateRunes(strings.TrimSpace(in.Author), uc.limits.MaxAuthorLength)
	profile.AvatarURL = strings.TrimSpace(in.AvatarURL)
	profile.FooterNote = truncateRunes(strings.TrimSpace(in.FooterNote), uc.limits.MaxFooterLength)
	profile.Wallpapers = string(wallpapers)
	profile.Socials = string(socials)

	if err := uc.store.SaveSiteProfile(ctx, profile); err != nil {
		return nil, internal("无法保存站点信息", err)
	}

	return uc.Get(ctx)
}

/* -------------------------------------------------------------------------- */
/* Helpers                                                                    */
/* -------------------------------------------------------------------------- */

func toSiteMetaDTO(profile model.SiteProfile, stats data.Stats) *SiteMetaDTO {
	// A malformed socials blob must not blank out the whole site shell; an
	// empty list is a far better failure than a 500 on every page.
	socials := make([]SocialLinkDTO, 0, 4)
	if raw := strings.TrimSpace(profile.Socials); raw != "" && raw != "[]" {
		var parsed []SocialLinkDTO
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			socials = parsed
		}
	}

	return &SiteMetaDTO{
		Title:       profile.Title,
		Tagline:     profile.Tagline,
		Description: profile.Description,
		Author:      profile.Author,
		AvatarURL:   profile.AvatarURL,
		Wallpapers:  parseWallpapers(profile.Wallpapers),
		Socials:     socials,
		FooterNote:  profile.FooterNote,
		Stats: SiteStatsDTO{
			PostCount:        stats.PostCount,
			TagCount:         stats.TagCount,
			WordCount:        stats.WordCount,
			FirstPublishedAt: rfc3339Ptr(stats.First),
			LastPublishedAt:  rfc3339Ptr(stats.Last),
		},
	}
}

// normaliseSocials trims, bounds and validates the social link list.
//
// Links that fail URL validation are dropped rather than stored: an entry that
// cannot be rendered safely is worse than an absent one, and the settings form
// shows the resulting list so the owner notices immediately.
func (uc *SiteUsecase) normaliseSocials(in []SocialLinkDTO) []SocialLinkDTO {
	out := make([]SocialLinkDTO, 0, len(in))

	for _, link := range in {
		if len(out) >= uc.limits.MaxSocials {
			break
		}

		target := strings.TrimSpace(link.URL)
		if target == "" || !isSafeLinkTarget(target) {
			continue
		}

		icon := strings.ToLower(strings.TrimSpace(link.Icon))
		if _, ok := knownSocialIcons[icon]; !ok {
			icon = "link"
		}

		label := truncateRunes(strings.TrimSpace(link.Label), uc.limits.MaxSocialLabelLength)
		if label == "" {
			label = icon
		}

		out = append(out, SocialLinkDTO{Icon: icon, Label: label, URL: target})
	}

	return out
}

// normaliseWallpapers trims, bounds, dedupes and validates the wallpaper pool.
//
// Order is preserved: the owner may deliberately put a favourite first, and the
// admin list is the only place that order is visible. Duplicates are dropped
// because a repeated URL would silently skew the random pick.
//
// A limit of 0 is not a bug: it is how the pool is switched off, and it yields
// an empty list rather than an error.
func (uc *SiteUsecase) normaliseWallpapers(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))

	for _, raw := range in {
		if len(out) >= uc.limits.MaxWallpapers {
			break
		}

		target := truncateRunes(strings.TrimSpace(raw), uc.limits.MaxWallpaperLength)
		if target == "" || !isSafeImageTarget(target) {
			continue
		}
		if _, dup := seen[target]; dup {
			continue
		}

		seen[target] = struct{}{}
		out = append(out, target)
	}

	return out
}

// parseWallpapers reads the stored JSON array back.
//
// A malformed blob yields an empty pool rather than an error: a site with no
// wallpaper is a far better failure than a 500 on every page load.
func parseWallpapers(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []string{}
	}

	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return []string{}
	}

	out := make([]string, 0, len(parsed))
	for _, item := range parsed {
		if target := strings.TrimSpace(item); target != "" {
			out = append(out, target)
		}
	}
	return out
}

// isSafeImageTarget allows site-relative paths and absolute http(s) URLs, and
// nothing else. It is deliberately narrower than isSafeLinkTarget: a wallpaper
// is painted by the browser as an image, so mailto: and javascript: can never be
// meaningful here.
func isSafeImageTarget(target string) bool {
	if strings.HasPrefix(target, "//") {
		return false
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}

	if parsed.Scheme == "" {
		return strings.HasPrefix(target, "/")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	default:
		return false
	}
}

// isSafeLinkTarget allows absolute http(s) and mailto links plus site-relative
// paths. Everything else — javascript:, data:, vbscript: — is refused.
func isSafeLinkTarget(target string) bool {
	if strings.HasPrefix(target, "//") {
		// Protocol-relative: the scheme is inherited, which is fine, but the
		// host is not ours, so require it to be a plain http(s) URL instead.
		return false
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}

	if parsed.Scheme == "" {
		return strings.HasPrefix(target, "/")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return parsed.Opaque != "" || parsed.Path != ""
	default:
		return false
	}
}
