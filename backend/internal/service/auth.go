package service

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

// meResponse wraps the user so the payload matches the frontend's
// `me(): Promise<{ user: User }>`.
type meResponse struct {
	User biz.UserDTO `json:"user"`
}

func (h *Handler) login(ctx khttp.Context) error {
	var input biz.CredentialsInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}
	result, err := h.auth.Login(ctx, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

// logout is a no-op server-side: tokens are stateless and the client discards
// its copy. Answering 204 rather than 401 means an already-expired session can
// still sign out cleanly.
func (h *Handler) logout(ctx khttp.Context) error {
	return noContent(ctx)
}

func (h *Handler) setupState(ctx khttp.Context) error {
	state, err := h.auth.SetupState(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, state)
}

func (h *Handler) setup(ctx khttp.Context) error {
	var input biz.CredentialsInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}
	result, err := h.auth.Setup(ctx, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

func (h *Handler) me(ctx khttp.Context, user *biz.CurrentUser) error {
	current, err := h.auth.Me(ctx, user.ID)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, meResponse{User: *current})
}

func (h *Handler) changePassword(ctx khttp.Context, user *biz.CurrentUser) error {
	var input biz.ChangePasswordInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}
	if err := h.auth.ChangePassword(ctx, user.ID, input); err != nil {
		return err
	}
	return noContent(ctx)
}
