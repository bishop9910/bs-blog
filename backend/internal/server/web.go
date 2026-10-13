package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/conf"
	"github.com/bishop9910/bs-blog/backend/internal/service"
)

// webHandler answers every request that no API route or uploaded file claimed.
//
// It has two jobs that look alike but must not be confused: unknown API paths
// get a JSON error so the frontend's ApiError machinery works, while unknown
// page paths get index.html so vue-router can resolve deep links.
type webHandler struct {
	root      string // absolute path of the built frontend; empty when disabled
	staticURL string // upload URL prefix, also never an SPA route
	log       *log.Helper
}

func newWebHandler(cfg *conf.Config, logger *log.Helper) *webHandler {
	root := strings.TrimSpace(cfg.Server.StaticDir)

	// A missing build directory is a perfectly normal state during development
	// and when running as an API-only server, so it is reported and skipped
	// rather than treated as a failure.
	switch abs, err := filepath.Abs(root); {
	case root == "":
	case err != nil:
		logger.Warnf("staticDir %q is unusable (%v); serving the API only", root, err)
	case !isFile(filepath.Join(abs, "index.html")):
		logger.Warnf("staticDir %q has no index.html; serving the API only", abs)
	default:
		root = abs
		logger.Infof("serving the built frontend from %s", abs)
	}

	return &webHandler{
		root:      root,
		staticURL: normalisePrefix(cfg.Data.UploadURL),
		log:       logger,
	}
}

func (w *webHandler) notFound(res http.ResponseWriter, req *http.Request) {
	if w.handlePreflight(res, req) {
		return
	}
	if w.isReserved(req.URL.Path) {
		khttp.DefaultErrorEncoder(res, req, kerrors.NotFound("NOT_FOUND", "接口不存在"))
		return
	}
	if w.root == "" {
		khttp.DefaultErrorEncoder(res, req, kerrors.NotFound("NOT_FOUND", "页面不存在"))
		return
	}
	w.servePage(res, req)
}

func (w *webHandler) methodNotAllowed(res http.ResponseWriter, req *http.Request) {
	if w.handlePreflight(res, req) {
		return
	}
	khttp.DefaultErrorEncoder(res, req,
		kerrors.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "请求方法不被允许"))
}

// handlePreflight closes out a CORS preflight. A preflight never matches a
// method-specific route, so it always lands in one of the two handlers above.
// The response headers were already added by the CORS filter.
func (w *webHandler) handlePreflight(res http.ResponseWriter, req *http.Request) bool {
	if req.Method != http.MethodOptions {
		return false
	}
	res.WriteHeader(http.StatusNoContent)
	return true
}

func (w *webHandler) isReserved(urlPath string) bool {
	if strings.HasPrefix(urlPath, service.APIPrefix) {
		return true
	}
	return w.staticURL != "" && strings.HasPrefix(urlPath, w.staticURL)
}

// servePage returns a real file when one exists and index.html otherwise.
func (w *webHandler) servePage(res http.ResponseWriter, req *http.Request) {
	// The leading slash before Clean is what makes this safe: every ".." is
	// resolved against the root, so the result can never escape it.
	relative := strings.TrimPrefix(path.Clean("/"+req.URL.Path), "/")

	if relative != "" {
		candidate := filepath.Join(w.root, filepath.FromSlash(relative))
		if isFile(candidate) {
			http.ServeFile(res, req, candidate)
			return
		}
	}

	http.ServeFile(res, req, filepath.Join(w.root, "index.html"))
}

func isFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

// noDirectoryListing hides the browseable listing http.FileServer would
// otherwise produce for a directory URL.
//
// The empty-path case is load-bearing: http.StripPrefix("/uploads/", ...) maps
// the bare "/uploads/" request to "", not to "/", so a trailing-slash test on
// its own lets through exactly the request that would print the listing.
func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if path := req.URL.Path; path == "" || strings.HasSuffix(path, "/") {
			http.NotFound(res, req)
			return
		}
		next.ServeHTTP(res, req)
	})
}
