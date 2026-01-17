package svc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/config"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlite "gorm.io/driver/sqlite"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

func openDatabase(cfg config.Config) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	dsn := strings.TrimSpace(cfg.Database.DataSource)

	// Allow env to override config for dev/CI.
	// See docs: DB_DRIVER, DATABASE_URL.
	if envDriver := strings.TrimSpace(os.Getenv("DB_DRIVER")); envDriver != "" {
		driver = strings.ToLower(envDriver)
	}
	if envDSN := strings.TrimSpace(os.Getenv("DATABASE_URL")); envDSN != "" {
		dsn = envDSN
	}

	if driver == "" {
		driver = "auto"
	}
	if driver == "auto" {
		switch {
		case strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://"):
			driver = "postgres"
		case strings.HasPrefix(dsn, "mysql://"):
			driver = "mysql"
		case strings.HasPrefix(dsn, "sqlserver://"):
			driver = "sqlserver"
		default:
			driver = "sqlite"
		}
	}

	switch driver {
	case "sqlite", "sqlite3":
		if dsn == "" {
			dsn = "data/croupier.db"
		}
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	case "postgres", "postgresql", "pg":
		if dsn == "" {
			return nil, fmt.Errorf("postgres DSN is required")
		}
		return gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	case "mysql":
		if dsn == "" {
			return nil, fmt.Errorf("mysql DSN is required")
		}
		return gorm.Open(gmysql.Open(dsn), &gorm.Config{})
	case "sqlserver":
		if dsn == "" {
			return nil, fmt.Errorf("sqlserver DSN is required")
		}
		return gorm.Open(gsqlserver.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func ensureSQLiteDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" {
		return nil
	}

	// Common SQLite DSNs:
	// - data/croupier.db
	// - file:data/croupier.db?cache=shared
	// - sqlite:///abs/path/to.db
	// - :memory:
	if strings.HasPrefix(dsn, "sqlite:///") {
		dsn = strings.TrimPrefix(dsn, "sqlite:///")
	}
	if strings.HasPrefix(dsn, "file:") {
		dsn = strings.TrimPrefix(dsn, "file:")
	}
	if idx := strings.IndexByte(dsn, '?'); idx >= 0 {
		dsn = dsn[:idx]
	}
	if dsn == "" || dsn == ":memory:" {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
