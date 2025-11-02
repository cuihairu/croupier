package db

import (
	"os"
	"path/filepath"
	"strings"

	gpostgres "gorm.io/driver/postgres"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open opens a gorm.DB using the given DSN. If dsn is empty, falls back to a local SQLite file.
// Supported DSN formats:
//   - postgres:  postgres://user:pass@host:5432/db?sslmode=disable
//   - sqlite:    sqlite:///path/to.db or file:path.db?cache=shared or :memory:
func Open(dsn string) (*gorm.DB, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://") {
		return gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	}
	if dsn == "" {
		// default to local sqlite under data/
		_ = os.MkdirAll("data", 0o755)
		dsn = filepath.ToSlash(filepath.Join("data", "croupier.db"))
		dsn = "file:" + dsn
	}
	if strings.HasPrefix(dsn, "sqlite:///") {
		dsn = "file:" + strings.TrimPrefix(dsn, "sqlite:///")
	}
	// sqlite forms: file:... or :memory:
	return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
}
