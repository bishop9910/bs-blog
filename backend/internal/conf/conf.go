// Package conf holds the on-disk configuration schema for the blog server.
//
// The file is intentionally small and human-editable: everything has a working
// default, so `backend/configs/config.yaml` may be deleted entirely and the
// server will still start.
package conf

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration lets the YAML file say `readTimeout: 60s` instead of a bare number
// of nanoseconds, which is what a plain time.Duration field would require.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if raw == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("conf: %q is not a valid duration: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

// Std converts back to a time.Duration for use with the standard library.
func (d Duration) Std() time.Duration { return time.Duration(d) }

// Config is the whole configuration tree.
type Config struct {
	Server ServerConfig `yaml:"server"`
	Data   DataConfig   `yaml:"data"`
	Auth   AuthConfig   `yaml:"auth"`
	CORS   CORSConfig   `yaml:"cors"`
	Log    LogConfig    `yaml:"log"`
}

// ServerConfig covers the HTTP listener.
type ServerConfig struct {
	// Addr is the listen address, e.g. ":8080" or "127.0.0.1:8080".
	Addr string `yaml:"addr"`
	// ReadTimeout bounds how long a request body may take to arrive.
	ReadTimeout Duration `yaml:"readTimeout"`
	// WriteTimeout must be generous: image uploads stream through it.
	WriteTimeout Duration `yaml:"writeTimeout"`
	// IdleTimeout is the keep-alive window.
	IdleTimeout Duration `yaml:"idleTimeout"`
	// MaxBodyBytes caps request bodies (uploads included).
	MaxBodyBytes int64 `yaml:"maxBodyBytes"`
	// StaticDir, when it exists on disk, is served as the built frontend. Deep
	// links fall through to index.html so the SPA router can resolve them.
	// Leave it empty to run as an API-only server.
	StaticDir string `yaml:"staticDir"`
}

// DataConfig points at storage on disk.
type DataConfig struct {
	// Database is the SQLite file path. The directory is created if missing.
	Database string `yaml:"database"`
	// UploadDir is where uploaded images are written.
	UploadDir string `yaml:"uploadDir"`
	// UploadURL is the public prefix those files are served under.
	UploadURL string `yaml:"uploadUrl"`
	// MaxUploadBytes caps a single image upload.
	MaxUploadBytes int64 `yaml:"maxUploadBytes"`
}

// AuthConfig configures the password lock.
type AuthConfig struct {
	// Secret is the JWT signing key. Left empty, a random secret is generated
	// once and persisted next to the database so tokens survive restarts.
	Secret string `yaml:"secret"`
	// SecretFile is where that generated secret is cached.
	SecretFile string `yaml:"secretFile"`
	// TokenTTL is how long a login lasts.
	TokenTTL Duration `yaml:"tokenTtl"`
	// BcryptCost is the password hashing work factor (bcrypt allows 4..31).
	BcryptCost int `yaml:"bcryptCost"`
	// SetupToken, when set, is required by POST /auth/setup so that a fresh
	// deployment cannot be claimed by a stranger. Optional.
	SetupToken string `yaml:"setupToken"`
}

// CORSConfig lists the browser origins allowed to call the API.
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

// LogConfig controls verbosity.
type LogConfig struct {
	// Level is one of: debug, info, warn, error.
	Level string `yaml:"level"`
}

// Default returns the configuration used when no file is present.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:         "127.0.0.1:8080",
			ReadTimeout:  Duration(60 * time.Second),
			WriteTimeout: Duration(120 * time.Second),
			IdleTimeout:  Duration(120 * time.Second),
			MaxBodyBytes: 12 << 20,
		},
		Data: DataConfig{
			Database:       "data/blog.db",
			UploadDir:      "data/uploads",
			UploadURL:      "/uploads",
			MaxUploadBytes: 10 << 20,
		},
		Auth: AuthConfig{
			SecretFile: "data/jwt.secret",
			TokenTTL:   Duration(7 * 24 * time.Hour),
			BcryptCost: 12,
		},
		CORS: CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{
				"http://localhost:5173",
				"http://127.0.0.1:5173",
			},
		},
		Log: LogConfig{Level: "info"},
	}
}

// Load reads path, falling back to defaults for a missing file. Any key that is
// absent keeps its default, so a partial file is valid.
func Load(path string) (*Config, error) {
	cfg := Default()

	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		// A missing config file is a supported state — defaults are complete.
		return cfg, nil
	case err != nil:
		return nil, fmt.Errorf("conf: reading %s: %w", path, err)
	}

	// Decode over the defaults so unspecified keys survive.
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("conf: parsing %s: %w", path, err)
	}
	return cfg, nil
}

// Validate reports nonsensical values early rather than at first request.
func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		return fmt.Errorf("conf: server.addr must not be empty")
	}
	if c.Data.Database == "" {
		return fmt.Errorf("conf: data.database must not be empty")
	}
	if c.Auth.BcryptCost < 4 || c.Auth.BcryptCost > 31 {
		return fmt.Errorf("conf: auth.bcryptCost must be between 4 and 31, got %d", c.Auth.BcryptCost)
	}
	if c.Auth.TokenTTL <= 0 {
		return fmt.Errorf("conf: auth.tokenTtl must be positive")
	}
	if c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("conf: server.maxBodyBytes must be positive")
	}
	return nil
}
