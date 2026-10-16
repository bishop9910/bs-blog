package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2"
	klog "github.com/go-kratos/kratos/v2/log"

	"github.com/bishop9910/bs-blog/backend/internal/biz"
	"github.com/bishop9910/bs-blog/backend/internal/conf"
	"github.com/bishop9910/bs-blog/backend/internal/data"
	"github.com/bishop9910/bs-blog/backend/internal/pkg/secure"
	"github.com/bishop9910/bs-blog/backend/internal/server"
	"github.com/bishop9910/bs-blog/backend/internal/service"
)

// version is stamped at build time:
//
//	go build -ldflags "-X main.version=1.0.0" ./cmd/server
var version = "dev"

func main() {
	confPath := flag.String("conf", "configs/config.yaml", "path to the YAML configuration file")
	flag.Parse()

	if err := run(*confPath); err != nil {
		fmt.Fprintf(os.Stderr, "bs-blog: %v\n", err)
		os.Exit(1)
	}
}

func run(confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	logger := klog.With(
		klog.NewStdLogger(os.Stdout),
		"ts", klog.DefaultTimestamp,
		"caller", klog.DefaultCaller,
		"service", "bs-blog",
	)
	klog.SetLogger(logger)
	helper := klog.NewHelper(logger)

	secret, err := signingSecret(cfg)
	if err != nil {
		return err
	}

	db, err := data.NewDB(cfg.Data.Database, cfg.Log.Level == "debug")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	store := data.NewStore(db)

	handler := service.NewHandler(
		biz.NewAuthUsecase(store, secure.NewSigner(secret, cfg.Auth.TokenTTL.Std()), cfg.Auth.BcryptCost),
		biz.NewPostUsecase(store),
		biz.NewMediaUsecase(store, cfg.Data.UploadDir, cfg.Data.UploadURL, cfg.Data.MaxUploadBytes),
		biz.NewSiteUsecase(store),
		biz.NewMusicUsecase(
			// The music library lives on disk rather than in the store, so it
			// takes the directory settings and a logger for the files it has to
			// skip while scanning.
			data.NewMusicRepo(cfg.Music.Dir, cfg.Music.Extensions, cfg.Music.Watch, cfg.Music.MaxUploadBytes, helper),
			cfg.Music.MaxUploadBytes,
		),
		cfg,
		logger,
	)

	app := kratos.New(
		kratos.Name("bs-blog"),
		kratos.Version(version),
		kratos.Server(server.NewHTTPServer(cfg, handler, helper)),
	)

	helper.Infof("bs-blog %s listening on %s", version, cfg.Server.Addr)
	return app.Run()
}

// signingSecret prefers an explicit hex-encoded secret from the configuration
// and otherwise loads (or creates) the key file. The key must be stable across
// restarts: a fresh one would invalidate every issued token.
func signingSecret(cfg *conf.Config) ([]byte, error) {
	if raw := strings.TrimSpace(cfg.Auth.Secret); raw != "" {
		secret, err := hex.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("auth.secret is not valid hex: %w", err)
		}
		if len(secret) < 32 {
			return nil, fmt.Errorf("auth.secret must decode to at least 32 bytes, got %d", len(secret))
		}
		return secret, nil
	}
	return secure.LoadOrCreateSecret(cfg.Auth.SecretFile)
}
