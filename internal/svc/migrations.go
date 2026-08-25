package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This file turns the legacy compensation hooks (see
// docs/architecture/database-migration-strategy.md, phase 3) into numbered
// goose Go migrations. They reuse the exact same implementations as the
// baseline bridge in internal/model so the two paths can never drift.
//
// Version layout:
//   0001 (SQL)  baseline marker
//   0002 (Go)   functions openapi column backfill
//   0003 (Go)   legacy table/index/column cleanup
//   0004 (Go)   varchar→int enum column conversion
//
// All three are dialect-probing and idempotent: on databases already healed by
// the baseline bridge they are no-ops. Down migrations only delete the version
// record (the cleanup/conversion is not reversible).

func init() {
	if err := goose.SetGlobalMigrations(
		openapiBackfillMigration(),
		legacyCleanupMigration(),
		enumColumnsMigration(),
	); err != nil {
		panic(fmt.Sprintf("svc: register goose go migrations: %v", err))
	}
}

func openapiBackfillMigration() *goose.Migration {
	return goose.NewGoMigration(2,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			return model.MigrateFunctionOpenAPIColumns(db)
		}},
		nil,
	)
}

func legacyCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(3,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if err := model.RenameLegacyTables(db); err != nil {
				return err
			}
			if err := model.DropLegacyPageUniqueIndexes(db); err != nil {
				return err
			}
			return model.CleanupAllLegacy(db)
		}},
		nil,
	)
}

func enumColumnsMigration() *goose.Migration {
	return goose.NewGoMigration(4,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			return model.MigrateEnumColumns(db)
		}},
		nil,
	)
}

// wrapGorm builds a *gorm.DB on top of the *sql.DB goose hands to Go
// migrations. The dialect is probed with cheap SQL pings because database/sql
// does not expose the driver name.
func wrapGorm(sqlDB *sql.DB) (*gorm.DB, error) {
	dialect, err := probeDialect(sqlDB)
	if err != nil {
		return nil, err
	}
	var dialector gorm.Dialector
	switch dialect {
	case "mysql":
		dialector = gmysql.New(gmysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true})
	case "postgres":
		dialector = gpostgres.New(gpostgres.Config{Conn: sqlDB})
	default:
		dialector = gsqlite.Dialector{Conn: sqlDB}
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("svc: wrap gorm for migration: %w", err)
	}
	return db, nil
}

func probeDialect(sqlDB *sql.DB) (string, error) {
	var n int
	// SQLite: sqlite_master exists only on SQLite (COUNT works on empty DBs).
	if err := sqlDB.QueryRow("SELECT COUNT(1) FROM sqlite_master").Scan(&n); err == nil {
		return "sqlite", nil
	}
	// Postgres: CURRENT_SETTING is not a MySQL function.
	var s string
	if err := sqlDB.QueryRow("SELECT CURRENT_SETTING('server_version')").Scan(&s); err == nil {
		return "postgres", nil
	}
	// MySQL: server-side version comment.
	if err := sqlDB.QueryRow("SELECT @@version_comment LIMIT 1").Scan(&s); err == nil {
		return "mysql", nil
	}
	return "", fmt.Errorf("svc: probe dialect: unsupported database")
}
