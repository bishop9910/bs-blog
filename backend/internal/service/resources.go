package service

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

// listPublicResources is the visitor-facing listing: visible files only.
func (h *Handler) listPublicResources(ctx khttp.Context) error {
	q := ctx.Query()
	result, err := h.resources.List(
		ctx,
		intArg(q.Get("page"), 1),
		intArg(q.Get("pageSize"), 0),
		q.Get("keyword"),
		true,
	)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

func (h *Handler) listResources(ctx khttp.Context, _ *biz.CurrentUser) error {
	q := ctx.Query()
	result, err := h.resources.List(
		ctx,
		intArg(q.Get("page"), 1),
		intArg(q.Get("pageSize"), 0),
		q.Get("keyword"),
		false,
	)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

// uploadResource takes one multipart body: the file plus the metadata typed
// into the admin form.
//
// The metadata rides along with the upload instead of being a second request,
// because a file whose title failed to save would otherwise sit in the list as
// an untitled orphan the owner has to find and edit.
func (h *Handler) uploadResource(ctx khttp.Context, _ *biz.CurrentUser) error {
	req := ctx.Request()

	// Small parts stay in memory and larger ones spill to temporary files, so an
	// upload never has to fit in RAM. The absolute ceiling comes from the
	// limitBody filter, which runs before this handler.
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
		return kerrors.BadRequest("INVALID_ARGUMENT", "请选择要上传的文件")
	}
	// The use case reopens the header itself, so this handle only needs closing.
	_ = file.Close()

	// Optional fields stay nil when the form leaves them out: the use case then
	// derives the title from the filename and defaults the rest.
	input := biz.ResourceInput{}
	if raw := strings.TrimSpace(req.FormValue("title")); raw != "" {
		input.Title = &raw
	}
	if raw := strings.TrimSpace(req.FormValue("description")); raw != "" {
		input.Description = &raw
	}
	if raw := strings.TrimSpace(req.FormValue("coverUrl")); raw != "" {
		input.CoverURL = &raw
	}
	// An absent field means "let the server decide" (visible); only an explicit
	// value overrides it.
	if raw := strings.TrimSpace(req.FormValue("visible")); raw != "" {
		visible := raw == "true" || raw == "1"
		input.Visible = &visible
	}

	asset, err := h.resources.Upload(ctx, header, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, asset)
}

func (h *Handler) updateResource(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	var input biz.ResourceInput
	if err := decodeBody(ctx, &input); err != nil {
		return err
	}

	asset, err := h.resources.Update(ctx, id, input)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, asset)
}

func (h *Handler) deleteResource(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	if err := h.resources.Delete(ctx, id); err != nil {
		return err
	}
	return noContent(ctx)
}

// downloadResource streams a file as an attachment.
//
// http.ServeContent does the parts that matter: Content-Length, Range for a
// resumable download and the HEAD body suppression. Everything it would
// otherwise infer is set here instead — the response is always an opaque
// attachment, so a browser can never decide to render an uploaded file.
func (h *Handler) downloadResource(ctx khttp.Context) error {
	id, err := pathID(ctx)
	if err != nil {
		return err
	}

	req := ctx.Request()
	// A HEAD probe must not inflate the counter.
	download, err := h.resources.Download(ctx, id, req.Method != http.MethodHead)
	if err != nil {
		return err
	}
	defer download.File.Close()

	res := ctx.Response()
	header := res.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", attachmentDisposition(download.Filename))
	header.Set("X-Content-Type-Options", "nosniff")
	// A download is one visitor's action, not a shared asset: letting a cache
	// hold it would also hand a stale file to the next visitor after a replace.
	header.Set("Cache-Control", "private, max-age=0, must-revalidate")

	http.ServeContent(res, req, download.Filename, download.ModTime, download.File)
	return nil
}

// attachmentDisposition builds the Content-Disposition value for a filename.
//
// mime.FormatMediaType emits the RFC 2231 form (the `filename*` parameter,
// UTF-8 encoded) whenever the name is not a plain token, which is what carries
// Chinese filenames intact. A quoted ASCII fallback is kept alongside it
// because older clients ignore the extended form and would otherwise save a
// file with no name at all.
func attachmentDisposition(name string) string {
	if formatted := mime.FormatMediaType("attachment", map[string]string{"filename": name}); formatted != "" {
		if fallback := asciiFilename(name); fallback != name {
			return fmt.Sprintf("%s; filename=%q", formatted, fallback)
		}
		return formatted
	}
	return fmt.Sprintf("attachment; filename=%q", asciiFilename(name))
}

// asciiFilename reduces a name to printable ASCII for the plain filename=
// parameter. The extension is what a user actually needs to keep, so it is
// preserved by simply copying the string.
func asciiFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" {
		return "download"
	}
	return out
}
