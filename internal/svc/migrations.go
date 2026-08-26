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
//   0005 (Go)   game-support context columns (docs/research/game-support-systems.md)
//   0006 (Go)   bug tracker baseline table (docs/research/bug-tracking-design.md)
//   0007 (Go)   tool registry baseline table (docs/research/tool-registry-design.md)

func init() {
	if err := goose.SetGlobalMigrations(
		openapiBackfillMigration(),
		legacyCleanupMigration(),
		enumColumnsMigration(),
		supportContextMigration(),
		bugTrackerMigration(),
		toolRegistryMigration(),
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

// supportContextColumns lists the game-support P1 columns added by migration
// 0005 (table model → column). GORM's AddColumn(model, columnName) emits the
// dialect-appropriate DDL from the model tags; HasColumn keeps it idempotent.
var supportContextColumns = []struct {
	model  func() interface{}
	column string
}{
	{func() interface{} { return &model.FAQ{} }, "slug"},
	{func() interface{} { return &model.FAQ{} }, "summary"},
	{func() interface{} { return &model.FAQ{} }, "helpful_count"},
	{func() interface{} { return &model.FAQ{} }, "unhelpful_count"},
	{func() interface{} { return &model.Ticket{} }, "server_id"},
	{func() interface{} { return &model.Ticket{} }, "player_level"},
	{func() interface{} { return &model.Ticket{} }, "device_os"},
	{func() interface{} { return &model.Ticket{} }, "device_model"},
	{func() interface{} { return &model.Ticket{} }, "language"},
	{func() interface{} { return &model.Ticket{} }, "extra"},
}

func supportContextMigration() *goose.Migration {
	return goose.NewGoMigration(5,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			for _, col := range supportContextColumns {
				mdl := col.model()
				table := mdl.(interface{ TableName() string }).TableName()
				if migrator.HasTable(table) && !migrator.HasColumn(mdl, col.column) {
					if err := migrator.AddColumn(mdl, col.column); err != nil {
						return fmt.Errorf("migrate: 0005 add %s.%s: %w", table, col.column, err)
					}
				}
			}
			return nil
		}},
		nil,
	)
}

// bugTrackerMigration creates the bugs table for the defect tracker
// (0006). HasTable keeps it idempotent; fresh databases already get the
// table from the AutoMigrate baseline.
func bugTrackerMigration() *goose.Migration {
	return goose.NewGoMigration(6,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.Bug{}) {
				if err := db.Migrator().CreateTable(&model.Bug{}); err != nil {
					return fmt.Errorf("migrate: 0006 create bugs: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// toolRegistryMigration creates the tool_links table (0007, toolbox).
func toolRegistryMigration() *goose.Migration {
	return goose.NewGoMigration(7,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.ToolLink{}) {
				if err := db.Migrator().CreateTable(&model.ToolLink{}); err != nil {
					return fmt.Errorf("migrate: 0007 create tool_links: %w", err)
				}
			}
			return nil
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
