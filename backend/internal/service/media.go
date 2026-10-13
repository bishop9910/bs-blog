package service

import (
	"errors"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

func (h *Handler) listMedia(ctx khttp.Context, _ *biz.CurrentUser) error {
	q := ctx.Query()
	result, err := h.media.List(ctx, intArg(q.Get("page"), 1), intArg(q.Get("pageSize"), 0), q.Get("keyword"))
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

func (h *Handler) uploadMedia(ctx khttp.Context, _ *biz.CurrentUser) error {
	req := ctx.Request()

	// Small parts stay in memory and larger ones spill to temporary files, so
	// an upload never has to fit in RAM. The absolute ceiling is applied by the
	// limitBody filter before this point.
	if err := req.ParseMultipartForm(4 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return kerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "上传内容过大")
		}
		return kerrors.BadRequest("INVALID_ARGUMENT", "上传数据无法解析")
	}
	defer func() {
		if req.MultipartForm != nil {
			_ = req.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := req.FormFile("file")
	if err != nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "请选择要上传的图片")
	}
	// The use case reopens the header itself, so this handle only needs closing.
	_ = file.Close()

	asset, err := h.media.Upload(ctx, header)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, asset)
}

func (h *Handler) deleteMedia(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	if err := h.media.Delete(ctx, id); err != nil {
		return err
	}
	return noContent(ctx)
}
