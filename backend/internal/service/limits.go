package service

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

// LimitsDTO is the effective upload and field policy of this deployment.
//
// It exists so the admin UI can quote the real numbers from config.yaml instead
// of hard-coding a second copy that drifts the moment an operator edits the
// file: the caps are enforced on write either way, and this endpoint is how the
// form learns what those caps are. It is a GET on the admin surface because the
// policy of a private deployment is not public information.
type LimitsDTO struct {
	Media     biz.MediaLimitsDTO    `json:"media"`
	Music     biz.MusicLimitsDTO    `json:"music"`
	Site      biz.SiteLimitsDTO     `json:"site"`
	Resources biz.ResourceLimitsDTO `json:"resources"`
}

func (h *Handler) getLimits(ctx khttp.Context, _ *biz.CurrentUser) error {
	return ctx.JSON(http.StatusOK, LimitsDTO{
		Media:     h.media.Limits(),
		Music:     h.music.Limits(),
		Site:      h.site.Limits(),
		Resources: h.resources.Limits(),
	})
}
