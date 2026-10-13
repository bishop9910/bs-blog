// Package data is the persistence layer: one Store per process, with the
// query methods split by aggregate across the files in this directory.
package data

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/bishop9910/bs-blog/backend/internal/model"
)

// Store owns the database handle. It is shared by every use case and is safe
// for concurrent use.
type Store struct {
	db *gorm.DB
}

// NewStore wraps an open database.
func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

// DB exposes the handle for the rare caller that needs it (health checks).
func (s *Store) DB() *gorm.DB { return s.db }

// NewDB opens the SQLite file at path and migrates the schema.
//
// Two choices here are load-bearing:
//
//   - CGO-free driver (glebarez/sqlite -> modernc.org/sqlite), so the binary is
//     a single static file with no gcc requirement on Windows.
//   - MaxOpenConns(1). SQLite permits a single writer, and a personal blog has
//     no throughput to lose; serialising here removes the entire class of
//     "database is locked" failures.
func NewDB(path string, verbose bool) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("data: creating database directory %s: %w", dir, err)
		}
	}

	// WAL keeps readers from blocking the writer; busy_timeout makes a
	// contended write wait rather than fail.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		filepath.ToSlash(path),
	)

	level := logger.Silent
	if verbose {
		level = logger.Warn
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		// Store UTC everywhere; the frontend formats into local time.
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("data: opening %s: %w", path, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("data: acquiring sql handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	// The DSN pragmas are the primary mechanism; these repeat them so a driver
	// that ignores unknown DSN parameters still ends up configured correctly.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	} {
		if err := db.Exec(pragma).Error; err != nil {
			return nil, fmt.Errorf("data: applying %q: %w", pragma, err)
		}
	}

	if err := db.AutoMigrate(model.All()...); err != nil {
		return nil, fmt.Errorf("data: migrating schema: %w", err)
	}

	return db, nil
}
