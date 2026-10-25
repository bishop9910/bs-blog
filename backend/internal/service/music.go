package service

import (
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
)

// musicIDPattern matches the 24 hex characters a track id is made of: the
// leading half of the SHA-256 of the track's path relative to the music
// directory.
//
// The music routes cannot reuse pathID, which parses the int64 ids the numbered
// models use. Checking the shape here keeps a malformed id out of the index
// lookup and answers with the same error envelope as every other route.
var musicIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// musicPathID reads the {id} route variable of a music route.
func musicPathID(ctx khttp.Context) (string, error) {
	id := strings.ToLower(strings.TrimSpace(ctx.Vars().Get("id")))
	if !musicIDPattern.MatchString(id) {
		return "", kerrors.BadRequest("INVALID_ARGUMENT", "无效的资源 ID")
	}
	return id, nil
}

// getMusicLibrary lists the library. It is public: the password lock protects
// writing, not listening.
func (h *Handler) getMusicLibrary(ctx khttp.Context) error {
	library, err := h.music.Library(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, library)
}

// streamMusic serves the audio bytes.
//
// http.ServeContent does the parts that matter for playback and would be easy to
// get wrong by hand: it answers a Range request with exactly the bytes the
// browser asked for (what makes seeking work), replies to HEAD, and honours
// If-Modified-Since. The file handle it needs comes from the index, so the id in
// the URL is the only thing a client controls.
func (h *Handler) streamMusic(ctx khttp.Context) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}

	file, track, err := h.music.OpenStream(ctx, id)
	if err != nil {
		return err
	}
	defer file.Close()

	res := ctx.Response()
	// A day's worth of seeks within one song must not re-hit the handler, and
	// the file's own mtime already tells a player when it changed.
	res.Header().Set("Cache-Control", "public, max-age=3600")
	res.Header().Set("Accept-Ranges", "bytes")
	if track.MIMEType != "" {
		res.Header().Set("Content-Type", track.MIMEType)
	}
	http.ServeContent(res, ctx.Request(), filepath.Base(track.Path), track.ModTime, file)
	return nil
}

// coverMusic serves a track's cover art. Missing art is a 404 in the API's usual
// JSON shape, which is exactly what the frontend falls back from when it renders
// a placeholder.
func (h *Handler) coverMusic(ctx khttp.Context) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}

	cover, err := h.music.OpenCover(ctx, id)
	if err != nil {
		return err
	}

	res := ctx.Response()
	res.Header().Set("Content-Type", cover.MIME)
	// Cover art never changes under an id — the id is the file's path — so it can
	// be cached for much longer than the audio.
	res.Header().Set("Cache-Control", "public, max-age=86400")
	res.Header().Set("Content-Length", strconv.Itoa(len(cover.Data)))
	res.WriteHeader(http.StatusOK)
	if ctx.Request().Method == http.MethodHead {
		return nil
	}
	// The status line is already out; a half-written image cannot be turned into
	// an error response any more, and the browser will simply retry.
	_, _ = res.Write(cover.Data)
	return nil
}

// musicLyrics returns one track's LRC text. A track without lyrics is a 200 with
// an empty string, not a 404: "no lyrics" is a normal state the page renders.
func (h *Handler) musicLyrics(ctx khttp.Context) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}

	lyrics, err := h.music.Lyrics(ctx, id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, lyrics)
}

// adminMusic describes the library for the settings page.
func (h *Handler) adminMusic(ctx khttp.Context, _ *biz.CurrentUser) error {
	library, err := h.music.Admin(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, library)
}

// rescanMusic rebuilds the index and answers with the fresh listing, so the page
// does not need a second request to show the result.
func (h *Handler) rescanMusic(ctx khttp.Context, _ *biz.CurrentUser) error {
	library, err := h.music.Rescan(ctx)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, library)
}

// uploadMusic stores one audio file and returns the track it became.
func (h *Handler) uploadMusic(ctx khttp.Context, _ *biz.CurrentUser) error {
	req := ctx.Request()

	// Small parts stay in memory and larger ones spill to temporary files, so an
	// upload never has to fit in RAM. The absolute ceiling is applied by the
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
		return kerrors.BadRequest("INVALID_ARGUMENT", "请选择要上传的音频文件")
	}
	// The use case reopens the header itself, so this handle only needs closing.
	_ = file.Close()

	// Lyrics are optional: the part is simply absent when the author uploads
	// only audio, which is the common case and not an error.
	var lyricsHeader *multipart.FileHeader
	if lyricsFile, lyricsPart, err := req.FormFile("lyrics"); err == nil {
		_ = lyricsFile.Close()
		lyricsHeader = lyricsPart
	} else if !errors.Is(err, http.ErrMissingFile) {
		return kerrors.BadRequest("INVALID_ARGUMENT", "歌词文件无法读取")
	}

	track, err := h.music.Save(ctx, header, lyricsHeader)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusCreated, track)
}

// setMusicLyrics replaces a track's .lrc with one the author picks now, and
// clearMusicLyrics removes it. Both answer with the refreshed track so the admin
// table can show the new state without a second request.
func (h *Handler) setMusicLyrics(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}

	req := ctx.Request()
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

	file, header, err := req.FormFile("lyrics")
	if err != nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "请选择要上传的歌词文件")
	}
	_ = file.Close()

	track, err := h.music.SetLyrics(ctx, id, header)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, track)
}

func (h *Handler) clearMusicLyrics(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}

	track, err := h.music.SetLyrics(ctx, id, nil)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, track)
}

// deleteMusic removes a track and its file from disk.
func (h *Handler) deleteMusic(ctx khttp.Context, _ *biz.CurrentUser) error {
	id, err := musicPathID(ctx)
	if err != nil {
		return err
	}
	if err := h.music.Delete(ctx, id); err != nil {
		return err
	}
	return noContent(ctx)
}
