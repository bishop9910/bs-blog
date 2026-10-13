package server

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/conf"
	"github.com/bishop9910/bs-blog/backend/internal/service"
	"github.com/bishop9910/bs-blog/backend/internal/web"
)

// webHandler answers every request that no API route or uploaded file claimed.
//
// It has two jobs that look alike but must not be confused: unknown API paths
// get a JSON error so the frontend's ApiError machinery works, while unknown
// page paths get index.html so vue-router can resolve deep links.
type webHandler struct {
	// files is the frontend tree: either the copy embedded in the binary or a
	// directory on disk. Both cases go through fs.FS, so everything below is
	// written once. A nil files means this process serves the API only.
	files fs.FS
	// assets memoises file bodies. Only names that actually exist are stored,
	// so a client inventing paths cannot grow the map.
	assets    sync.Map // string -> []byte
	staticURL string   // upload URL prefix, also never an SPA route
	log       *log.Helper
}

func newWebHandler(cfg *conf.Config, logger *log.Helper) *webHandler {
	return &webHandler{
		files:     frontendFS(strings.TrimSpace(cfg.Server.StaticDir), logger),
		staticURL: normalisePrefix(cfg.Data.UploadURL),
		log:       logger,
	}
}

// frontendFS decides what serves the pages.
//
// The frontend is embedded in the binary, so the normal deployment carries no
// dist directory at all. An explicit staticDir still wins when it is usable,
// which is what makes it possible to iterate on the frontend against a built
// binary; a configured path that is missing or holds no index.html falls back
// to the embedded copy rather than disabling pages outright.
//
// Returning an fs.FS rather than a path keeps the serving code below identical
// for both sources -- and sidesteps os.DirFS-style path handling mistakes,
// since embed.FS and os.DirFS both reject a path that escapes the root.
func frontendFS(staticDir string, logger *log.Helper) fs.FS {
	if staticDir != "" {
		abs, err := filepath.Abs(staticDir)
		switch {
		case err != nil:
			logger.Warnf("staticDir %q is unusable (%v); using the embedded frontend", staticDir, err)
		case !isFile(filepath.Join(abs, "index.html")):
			logger.Warnf("staticDir %q has no index.html; using the embedded frontend", abs)
		default:
			logger.Infof("serving the frontend from %s", abs)
			return os.DirFS(abs)
		}
	}

	if !web.Built() {
		logger.Warn("this binary carries no frontend and no usable staticDir; serving the API only")
		return nil
	}
	logger.Infof("serving the frontend embedded in this binary")
	return web.Dist()
}

func (w *webHandler) notFound(res http.ResponseWriter, req *http.Request) {
	if w.handlePreflight(res, req) {
		return
	}
	if w.isReserved(req.URL.Path) {
		khttp.DefaultErrorEncoder(res, req, kerrors.NotFound("NOT_FOUND", "接口不存在"))
		return
	}
	if w.files == nil {
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
		if body, ok := w.asset(relative); ok {
			w.writeAsset(res, req, relative, body)
			return
		}
	}

	body, ok := w.asset("index.html")
	if !ok {
		khttp.DefaultErrorEncoder(res, req, kerrors.NotFound("NOT_FOUND", "页面不存在"))
		return
	}
	w.writeAsset(res, req, "index.html", body)
}

// writeAsset sends one frontend file.
//
// The modtime is deliberately zero. Embedded files have no timestamp, and a
// download directory can be updated under a running server, so claiming a
// modification time would only produce wrong 304s. Freshness is expressed
// through Cache-Control instead, where the Vite content hashes make it exact.
func (w *webHandler) writeAsset(res http.ResponseWriter, req *http.Request, name string, body []byte) {
	setCacheControl(res, name)
	// bytes.Reader is what gives ServeContent the seeker it needs for HEAD and
	// range requests; passing a plain reader would silently drop both.
	http.ServeContent(res, req, path.Base(name), time.Time{}, bytes.NewReader(body))
}

// setCacheControl splits the frontend into the two halves that want opposite
// treatment.
func setCacheControl(res http.ResponseWriter, name string) {
	switch {
	case name == "index.html":
		// Never cache the entry point: it is the file that names the hashed
		// bundles, so a stale copy pins the browser to a stale build.
		res.Header().Set("Cache-Control", "no-cache")
	case strings.HasPrefix(name, "assets/"):
		// Vite puts a content hash in every bundle filename: a changed file is
		// a changed URL, so these can be cached forever.
		res.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		// Everything else (favicon, fonts, robots.txt) keeps a name across
		// builds, so it gets a bounded lifetime and revalidates after that.
		res.Header().Set("Cache-Control", "public, max-age=3600")
	}
}

// asset reads one frontend file and remembers it. Holding the bytes is close to
// free: the whole embedded tree is already mapped into the process image.
func (w *webHandler) asset(name string) ([]byte, bool) {
	if cached, ok := w.assets.Load(name); ok {
		return cached.([]byte), true
	}
	// ValidPath rejects absolute paths and any ".." element, so a request can
	// never address a file outside the frontend tree.
	if !fs.ValidPath(name) {
		return nil, false
	}

	file, err := w.files.Open(name)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return nil, false
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return nil, false
	}

	w.assets.Store(name, body)
	return body, true
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
