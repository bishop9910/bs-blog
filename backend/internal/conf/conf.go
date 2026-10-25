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
	Site   SiteConfig   `yaml:"site"`
	Music  MusicConfig  `yaml:"music"`
	// Resources is the download library on the resources page. It is a separate
	// directory from data.uploadDir on purpose: images are served inline by the
	// static handler, while these files are only ever handed out as an
	// attachment through the API, so the two must not share a directory or a
	// path could be served the wrong way.
	Resources ResourceConfig `yaml:"resources"`
	Auth      AuthConfig     `yaml:"auth"`
	CORS      CORSConfig     `yaml:"cors"`
	Log       LogConfig      `yaml:"log"`
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
	// StaticDir overrides the frontend embedded in the binary. Empty means
	// "serve the embedded copy", which is what makes a release a single
	// executable. A path that is missing or holds no index.html also falls back
	// to the embedded copy, so a stale setting cannot take the site down.
	// Point it at a real directory (e.g. "../dist") to iterate on the frontend
	// without rebuilding Go. Deep links fall through to index.html so the SPA
	// router can resolve them, and a binary carrying no frontend serves the API
	// only.
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

// SiteConfig bounds the site metadata the settings page may store.
//
// Only the fields worth tuning per deployment live here. The per-field text
// caps (title, tagline, description, …) stay in internal/biz/site.go because
// they are matched to the column sizes in internal/model; the settings UI still
// reads them back through GET /admin/limits, so it never has to guess.
type SiteConfig struct {
	// MaxWallpapers caps the wallpaper pool. 0 turns wallpapers off entirely,
	// which is a supported state: the site falls back to the shader backdrop.
	MaxWallpapers int `yaml:"maxWallpapers"`
	// MaxWallpaperLength caps one wallpaper URL, in characters. Wallpapers may
	// be absolute links, so this is generous.
	MaxWallpaperLength int `yaml:"maxWallpaperLength"`
}

// MusicConfig points at the on-disk music library.
//
// The library is the directory: no track is ever written to SQLite, so moving or
// syncing the folder is all it takes to change what the site plays.
type MusicConfig struct {
	// Dir is the directory scanned for audio files, recursively. Empty disables
	// the library: the routes stay registered and answer with an empty listing,
	// which is friendlier than a 404 on a page that is part of the site.
	Dir string `yaml:"dir"`
	// Extensions are the file extensions indexed, without the leading dot. An
	// empty list falls back to the formats the audio package can read.
	Extensions []string `yaml:"extensions"`
	// Watch rescans a couple of seconds after the directory changes, so dropping
	// files in by hand shows up without pressing rescan.
	Watch bool `yaml:"watch"`
	// MaxUploadBytes caps a single audio upload. Must stay below
	// server.maxBodyBytes, which is the ceiling that actually reaches the socket.
	MaxUploadBytes int64 `yaml:"maxUploadBytes"`
	// MaxCoverBytes caps a cover image read off disk, whether it sits inside the
	// audio file's tags or in a sidecar. Measured on the file, before reading.
	MaxCoverBytes int64 `yaml:"maxCoverBytes"`
	// MaxLyricsBytes caps a .lrc file. Lyrics are kilobytes in practice; the
	// ceiling only stops a mistaken path from being read into memory.
	MaxLyricsBytes int64 `yaml:"maxLyricsBytes"`
}

// ResourceConfig points at the on-disk collection of downloadable files.
//
// Unlike the music library, the directory is required: the resources page is
// the point of the feature, and a silently empty page would look like the
// upload failed. Files are always streamed back through the API as an
// attachment, never served from a static prefix, which is why there is no
// uploadUrl counterpart here.
type ResourceConfig struct {
	// Dir is where uploaded files are written.
	Dir string `yaml:"dir"`
	// Extensions are the file suffixes accepted, without the leading dot and
	// case-insensitively. An empty list falls back to the set internal/biz
	// accepts, so the allow-list has one authoritative definition in code.
	Extensions []string `yaml:"extensions"`
	// MaxUploadBytes caps one file. Must not exceed server.maxBodyBytes, which
	// is the ceiling the request body filter enforces before any handler runs.
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
	// SetupToken is reserved for requiring a shared secret on POST /auth/setup,
	// so that a fresh deployment cannot be claimed by whichever stranger reaches
	// the port first. It is NOT enforced yet — no handler reads it — so it is
	// deliberately absent from configs/config.yaml: a value there would sit in a
	// git-tracked file looking like a protection that does not exist.
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
			MaxBodyBytes: 32 << 20,
		},
		Data: DataConfig{
			Database:       "data/blog.db",
			UploadDir:      "data/uploads",
			UploadURL:      "/uploads",
			MaxUploadBytes: 10 << 20,
		},
		Site: SiteConfig{
			MaxWallpapers:      24,
			MaxWallpaperLength: 512,
		},
		Music: MusicConfig{
			Dir:            "data/music",
			Extensions:     []string{"mp3", "flac", "m4a", "m4b", "ogg", "opus", "wav"},
			Watch:          true,
			MaxUploadBytes: 32 << 20,
			MaxCoverBytes:  5 << 20,
			MaxLyricsBytes: 1 << 20,
		},
		Resources: ResourceConfig{
			Dir: "data/resources",
			// Mirrors server.maxBodyBytes above: the body filter would reject a
			// larger upload before the handler could explain itself.
			MaxUploadBytes: 32 << 20,
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
	if c.Data.MaxUploadBytes <= 0 {
		return fmt.Errorf("conf: data.maxUploadBytes must be positive")
	}
	if c.Data.MaxUploadBytes > c.Server.MaxBodyBytes {
		// Same trap as the music ceiling below: the body filter runs first, so a
		// larger per-image cap could never be reached and the upload would fail
		// with "request too large" from an unrelated layer.
		return fmt.Errorf(
			"conf: data.maxUploadBytes (%d) must not exceed server.maxBodyBytes (%d)",
			c.Data.MaxUploadBytes, c.Server.MaxBodyBytes,
		)
	}
	if c.Site.MaxWallpapers < 0 {
		return fmt.Errorf("conf: site.maxWallpapers must not be negative")
	}
	// 0 wallpapers is a supported state (the feature is off), so the length only
	// has to make sense while the pool is actually allowed to hold something.
	if c.Site.MaxWallpapers > 0 && c.Site.MaxWallpaperLength <= 0 {
		return fmt.Errorf("conf: site.maxWallpaperLength must be positive while wallpapers are enabled")
	}
	// The resources directory is required: without it every upload fails, and
	// the failure would only surface after a multipart body had been read.
	if c.Resources.Dir == "" {
		return fmt.Errorf("conf: resources.dir must not be empty")
	}
	if c.Resources.MaxUploadBytes <= 0 {
		return fmt.Errorf("conf: resources.maxUploadBytes must be positive")
	}
	if c.Resources.MaxUploadBytes > c.Server.MaxBodyBytes {
		return fmt.Errorf(
			"conf: resources.maxUploadBytes (%d) must not exceed server.maxBodyBytes (%d)",
			c.Resources.MaxUploadBytes, c.Server.MaxBodyBytes,
		)
	}
	// An empty music.dir is a supported state — the library is simply disabled —
	// so the rest of the block is only meaningful once a directory is named.
	if c.Music.Dir != "" {
		if c.Music.MaxUploadBytes <= 0 {
			return fmt.Errorf("conf: music.maxUploadBytes must be positive")
		}
		if c.Music.MaxCoverBytes <= 0 {
			return fmt.Errorf("conf: music.maxCoverBytes must be positive")
		}
		if c.Music.MaxLyricsBytes <= 0 {
			return fmt.Errorf("conf: music.maxLyricsBytes must be positive")
		}
		if c.Server.MaxBodyBytes < c.Music.MaxUploadBytes {
			// The body filter rejects the request before the upload handler can
			// explain itself, so a ceiling larger than the filter would only ever
			// produce "request too large" from an unrelated layer.
			return fmt.Errorf(
				"conf: music.maxUploadBytes (%d) must not exceed server.maxBodyBytes (%d)",
				c.Music.MaxUploadBytes, c.Server.MaxBodyBytes,
			)
		}
	}
	return nil
}
