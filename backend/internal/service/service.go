// Package service exposes the blog's HTTP API.
//
// Routes are registered by hand rather than generated from protobuf, which
// keeps the wire shape under direct control — the frontend types in
// src/types/blog.ts are the contract — and keeps a code-generation step out of
// the build. Kratos still supplies the router, the error model and the filter
// plumbing.
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
	"github.com/bishop9910/bs-blog/backend/internal/conf"
)

// APIPrefix is the mount point of the JSON API. The frontend's API_BASE
// defaults to the same value.
const APIPrefix = "/api/v1"

// Handler holds the use cases the routes delegate to.
type Handler struct {
	auth  *biz.AuthUsecase
	posts *biz.PostUsecase
	media *biz.MediaUsecase
	site  *biz.SiteUsecase
	music *biz.MusicUsecase

	maxBody        int64
	corsEnabled    bool
	allowAnyOrigin bool
	allowedOrigins map[string]struct{}
	logger         *log.Helper
}

// NewHandler wires the HTTP layer to the use cases.
func NewHandler(
	auth *biz.AuthUsecase,
	posts *biz.PostUsecase,
	media *biz.MediaUsecase,
	site *biz.SiteUsecase,
	music *biz.MusicUsecase,
	cfg *conf.Config,
	logger log.Logger,
) *Handler {
	allowed := make(map[string]struct{}, len(cfg.CORS.AllowedOrigins))
	allowAny := false
	for _, origin := range cfg.CORS.AllowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		switch origin {
		case "":
			continue
		case "*":
			allowAny = true
		default:
			allowed[origin] = struct{}{}
		}
	}

	return &Handler{
		auth:           auth,
		posts:          posts,
		media:          media,
		site:           site,
		music:          music,
		maxBody:        cfg.Server.MaxBodyBytes,
		corsEnabled:    cfg.CORS.Enabled,
		allowAnyOrigin: allowAny,
		allowedOrigins: allowed,
		logger:         log.NewHelper(logger),
	}
}

// Filters returns the server-wide filters, outermost first.
//
// Kratos applies ServerOptions of type Filter around the whole router, so
// these cover every route including unmatched ones — unlike Middleware, which
// never reaches handlers registered through Route().
func (h *Handler) Filters() []khttp.FilterFunc {
	filters := []khttp.FilterFunc{h.recoverPanic}
	if h.corsEnabled {
		filters = append(filters, h.cors)
	}
	return append(filters, h.logRequest, h.limitBody)
}

// Register attaches every API route to the server.
func (h *Handler) Register(srv *khttp.Server) {
	r := srv.Route(APIPrefix)

	// Public reading surface.
	h.route(r, http.MethodGet, "/posts", h.listPosts)
	h.route(r, http.MethodGet, "/posts/{slug}", h.getPost)
	h.route(r, http.MethodGet, "/archive", h.getArchive)
	h.route(r, http.MethodGet, "/tags", h.listTags)
	h.route(r, http.MethodGet, "/site", h.getSite)

	// Session. Only the two routes that return or change owner data are
	// guarded; login and setup must be reachable while signed out.
	h.route(r, http.MethodPost, "/auth/login", h.login)
	h.route(r, http.MethodGet, "/auth/setup", h.setupState)
	h.route(r, http.MethodPost, "/auth/setup", h.setup)
	h.route(r, http.MethodPost, "/auth/logout", h.logout)
	h.route(r, http.MethodGet, "/auth/me", h.authed(h.me))
	h.route(r, http.MethodPost, "/auth/password", h.authed(h.changePassword))

	// Admin: posts.
	h.route(r, http.MethodGet, "/admin/posts", h.authed(h.listAdminPosts))
	h.route(r, http.MethodPost, "/admin/posts", h.authed(h.createPost))
	h.route(r, http.MethodGet, "/admin/posts/{id}", h.authed(h.getAdminPost))
	h.route(r, http.MethodPut, "/admin/posts/{id}", h.authed(h.updatePost))
	h.route(r, http.MethodDelete, "/admin/posts/{id}", h.authed(h.deletePost))
	h.route(r, http.MethodPost, "/admin/posts/{id}/publish", h.authed(h.publishPost))
	h.route(r, http.MethodPost, "/admin/posts/{id}/unpublish", h.authed(h.unpublishPost))
	h.route(r, http.MethodGet, "/admin/slug", h.authed(h.checkSlug))

	// Admin: media library.
	h.route(r, http.MethodGet, "/admin/media", h.authed(h.listMedia))
	h.route(r, http.MethodPost, "/admin/media", h.authed(h.uploadMedia))
	h.route(r, http.MethodDelete, "/admin/media/{id}", h.authed(h.deleteMedia))

	// Public music library. Listening needs no session: the audio routes carry
	// no token, which is what lets an <audio> element and the Web Audio API read
	// the same bytes the page's own origin served.
	h.route(r, http.MethodGet, "/music/library", h.getMusicLibrary)
	h.route(r, http.MethodGet, "/music/stream/{id}", h.streamMusic)
	h.route(r, http.MethodHead, "/music/stream/{id}", h.streamMusic)
	h.route(r, http.MethodGet, "/music/cover/{id}", h.coverMusic)
	h.route(r, http.MethodHead, "/music/cover/{id}", h.coverMusic)
	h.route(r, http.MethodGet, "/music/tracks/{id}/lyrics", h.musicLyrics)

	// Admin: music library.
	h.route(r, http.MethodGet, "/admin/music", h.authed(h.adminMusic))
	h.route(r, http.MethodPost, "/admin/music/rescan", h.authed(h.rescanMusic))
	h.route(r, http.MethodPost, "/admin/music/upload", h.authed(h.uploadMusic))
	h.route(r, http.MethodDelete, "/admin/music/{id}", h.authed(h.deleteMusic))

	// Admin: site metadata.
	h.route(r, http.MethodPut, "/admin/site", h.authed(h.updateSite))

	// Admin: the effective size and field policy, so the forms quote the values
	// this deployment actually enforces rather than a hard-coded copy.
	h.route(r, http.MethodGet, "/admin/limits", h.authed(h.getLimits))
}

func (h *Handler) route(r *khttp.Router, method, pattern string, fn khttp.HandlerFunc) {
	r.Handle(method, pattern, fn)
}

/* -------------------------------------------------------------------------- */
/* Authentication plumbing                                                    */
/* -------------------------------------------------------------------------- */

// authedFunc is a handler that requires the signed-in owner.
type authedFunc func(ctx khttp.Context, user *biz.CurrentUser) error

// authed wraps a handler so the credential check happens in the handler body
// rather than in a router filter. It is deliberately explicit: a new protected
// route cannot silently end up unguarded, because there is no way to reach the
// inner handler without going through currentUser first.
func (h *Handler) authed(fn authedFunc) khttp.HandlerFunc {
	return func(ctx khttp.Context) error {
		user, err := h.currentUser(ctx)
		if err != nil {
			return err
		}
		return fn(ctx, user)
	}
}

func (h *Handler) currentUser(ctx khttp.Context) (*biz.CurrentUser, error) {
	raw := strings.TrimSpace(ctx.Request().Header.Get("Authorization"))
	token := raw
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		token = strings.TrimSpace(raw[7:])
	}
	if token == "" {
		return nil, kerrors.Unauthorized("UNAUTHORIZED", "请先登录")
	}
	return h.auth.Verify(ctx, token)
}

/* -------------------------------------------------------------------------- */
/* Request helpers                                                            */
/* -------------------------------------------------------------------------- */

// decodeBody reads a JSON request body.
//
// Bind is not used because its error mapping is coarse, and an oversized body
// has to be distinguishable from malformed JSON.
func decodeBody(ctx khttp.Context, target any) error {
	body := ctx.Request().Body
	if body == nil {
		return kerrors.BadRequest("INVALID_ARGUMENT", "请求内容为空")
	}

	if err := json.NewDecoder(body).Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return kerrors.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "请求内容过大")
		}
		if errors.Is(err, io.EOF) {
			return kerrors.BadRequest("INVALID_ARGUMENT", "请求内容为空")
		}
		return kerrors.BadRequest("INVALID_ARGUMENT", "请求内容格式不正确")
	}
	return nil
}

// noContent writes a 204 with a genuinely empty body.
func noContent(ctx khttp.Context) error {
	ctx.Response().WriteHeader(http.StatusNoContent)
	return nil
}

// queryOf maps the shared collection query parameters.
//
// The frontend strips status=all before sending, but a stale client or a
// hand-typed URL may still include it, so it is normalised away here too.
func queryOf(ctx khttp.Context) biz.Query {
	q := ctx.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status == "all" {
		status = ""
	}
	return biz.Query{
		Page:     intArg(q.Get("page"), 1),
		PageSize: intArg(q.Get("pageSize"), 0),
		Status:   status,
		Tag:      strings.TrimSpace(q.Get("tag")),
		Keyword:  strings.TrimSpace(q.Get("keyword")),
		Sort:     strings.TrimSpace(q.Get("sort")),
		Order:    strings.TrimSpace(q.Get("order")),
	}
}

func intArg(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// pathID reads the {id} route variable.
func pathID(ctx khttp.Context) (int64, error) {
	raw := strings.TrimSpace(ctx.Vars().Get("id"))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, kerrors.BadRequest("INVALID_ARGUMENT", "无效的资源 ID")
	}
	return id, nil
}

/* -------------------------------------------------------------------------- */
/* Filters                                                                    */
/* -------------------------------------------------------------------------- */

func (h *Handler) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			// net/http raises this sentinel on purpose to abort a response;
			// swallowing it would hide an intentional shutdown.
			if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(recovered)
			}
			h.logger.Errorf("panic serving %s %s: %v\n%s", req.Method, req.URL.Path, recovered, debug.Stack())
			khttp.DefaultErrorEncoder(res, req, kerrors.InternalServer("INTERNAL_ERROR", "服务器内部错误"))
		}()
		next.ServeHTTP(res, req)
	})
}

func (h *Handler) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		origin := strings.TrimSpace(req.Header.Get("Origin"))
		if origin != "" && h.originAllowed(origin) {
			header := res.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Add("Vary", "Origin")
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			header.Set("Access-Control-Max-Age", "600")
		}
		next.ServeHTTP(res, req)
	})
}

func (h *Handler) originAllowed(origin string) bool {
	if h.allowAnyOrigin {
		return true
	}
	_, ok := h.allowedOrigins[strings.TrimRight(origin, "/")]
	return ok
}

func (h *Handler) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		started := time.Now()
		tracked := &statusWriter{ResponseWriter: res}
		next.ServeHTTP(tracked, req)

		message := fmt.Sprintf("%s %s %d %s", req.Method, req.URL.Path, tracked.status(), time.Since(started).Round(time.Millisecond))
		if tracked.status() >= http.StatusInternalServerError {
			h.logger.Error(message)
			return
		}
		h.logger.Info(message)
	})
}

// limitBody caps request bodies before any handler reads them.
//
// Kratos exposes no body-size option at all, so this filter is the only place
// the limit can be enforced.
func (h *Handler) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.Body != nil && h.maxBody > 0 {
			req.Body = http.MaxBytesReader(res, req.Body, h.maxBody)
		}
		next.ServeHTTP(res, req)
	})
}

// statusWriter records the response status so it can be logged.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

func (w *statusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Flush keeps streaming responses working through the wrapper.
func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
