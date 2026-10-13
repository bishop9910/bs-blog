package service

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

func (h *Handler) getSite(ctx khttp.Context) error {
	meta, err := h.site.Get(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, meta)
}

func (h *Handler) updateSite(ctx khttp.Context, _ *biz.CurrentUser) error {
	// The frontend sends a SiteMeta (its type is Partial, but every field is
	// populated), so unknown keys such as stats are simply ignored by the
	// decoder and the response carries the authoritative values back.
	var input biz.SiteInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}

	meta, err := h.site.Update(ctx, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, meta)
}
