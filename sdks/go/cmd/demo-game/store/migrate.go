package store

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// platformDBNames are the platform-owned databases. The demo game must never
// open them: demo tables (demo_*) would leak into the platform schema and a
// reseed would be catastrophic. Open refuses such DSNs outright.
var platformDBNames = map[string]bool{
	"croupier":      true,
	"croupier_meta": true,
}

// Open picks the backend by DSN shape:
//   - ""                        → memoryBackend (process-lifetime only)
//   - postgres:// or postgresql:// → sqlBackend over pgx
//   - anything else             → sqlBackend over pure-Go sqlite (glebarez, CGO-free)
//
// The platform-DB guard runs before any connection attempt.
func Open(dsn string) (Backend, error) {
	if dsn == "" {
		return newMemoryBackend(), nil
	}
	name, err := DatabaseName(dsn)
	if err != nil {
		return nil, err
	}
	if platformDBNames[name] {
		return nil, fmt.Errorf("demo-game refuses DSN pointing at platform database %q; "+
			"give the demo its own database (e.g. demo_game)", name)
	}
	var db *gorm.DB
	var err2 error
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		db, err2 = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else {
		db, err2 = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	}
	if err2 != nil {
		return nil, fmt.Errorf("open demo store (%s): %w", name, err2)
	}
	if err := db.AutoMigrate(allModels()...); err != nil {
		return nil, fmt.Errorf("migrate demo store: %w", err)
	}
	return &sqlBackend{db: db}, nil
}

// DatabaseName extracts the database identifier from a DSN. URL-form DSNs
// (postgres://host/db?...) parse as URLs; sqlite paths use the file base
// name without extension; keyword/value DSNs use dbname=. Exported for the
// guard tests.
func DatabaseName(dsn string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parse DSN: %w", err)
		}
		return strings.TrimPrefix(u.Path, "/"), nil
	}
	// keyword/value form (postgres): dbname=xxx
	for _, field := range strings.Fields(dsn) {
		if kv := strings.SplitN(field, "=", 2); len(kv) == 2 && kv[0] == "dbname" {
			return kv[1], nil
		}
	}
	// sqlite: file path or file: URI — use the base name without extension.
	path := dsn
	if rest, ok := strings.CutPrefix(dsn, "file:"); ok {
		// file: URI may carry query params after the path.
		if q := strings.IndexByte(rest, '?'); q >= 0 {
			rest = rest[:q]
		}
		path = rest
	}
	base := filepath.Base(path)
	if base == "." || base == "/" || base == ":memory:" {
		return "", nil
	}
	return strings.TrimSuffix(base, filepath.Ext(base)), nil
}
