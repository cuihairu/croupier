package svc

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/config"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlite "gorm.io/driver/sqlite"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

func openDatabase(cfg config.Config) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Server.Database.Driver))
	dsn := strings.TrimSpace(cfg.Server.Database.DataSource)

	if driver == "" {
		driver = "sqlite"
	}

	switch driver {
	case "sqlite", "sqlite3":
		if dsn == "" {
			dsn = "data/croupier.db"
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
