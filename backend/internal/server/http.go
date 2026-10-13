package server

import (
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/bishop9910/bs-blog/backend/internal/conf"
	"github.com/bishop9910/bs-blog/backend/internal/service"
)

// NewHTTPServer assembles the Kratos HTTP transport.
func NewHTTPServer(cfg *conf.Config, handler *service.Handler, logger *log.Helper) *khttp.Server {
	web := newWebHandler(cfg, logger)

	srv := khttp.NewServer(
		khttp.Address(cfg.Server.Addr),

		// Kratos defaults to a one second request deadline, which would cut off
		// every image upload and any slow write. Zero switches the transport to
		// plain cancellation, leaving the socket timeouts below in charge.
		khttp.Timeout(0),

		// Filters wrap the entire router, unlike Middleware — which never
		// reaches handlers registered through Route().
		khttp.Filter(handler.Filters()...),

		// Leaving these at the kratos default would hand unknown paths to
		// http.DefaultServeMux. Ours answers JSON for the API and the SPA shell
		// for everything else.
		khttp.NotFoundHandler(http.HandlerFunc(web.notFound)),
		khttp.MethodNotAllowedHandler(http.HandlerFunc(web.methodNotAllowed)),
	)

	// The embedded http.Server's timeout fields have no kratos option.
	srv.ReadTimeout = cfg.Server.ReadTimeout.Std()
	srv.WriteTimeout = cfg.Server.WriteTimeout.Std()
	srv.IdleTimeout = cfg.Server.IdleTimeout.Std()

	handler.Register(srv)

	if prefix := normalisePrefix(cfg.Data.UploadURL); prefix != "" && cfg.Data.UploadDir != "" {
		files := http.StripPrefix(prefix+"/", noDirectoryListing(http.FileServer(http.Dir(cfg.Data.UploadDir))))
		srv.HandlePrefix(prefix+"/", files)
	}

	return srv
}

// normalisePrefix turns "uploads" or "/uploads/" into "/uploads" and an empty
// or root value into "".
func normalisePrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	return "/" + strings.Trim(value, "/")
}
