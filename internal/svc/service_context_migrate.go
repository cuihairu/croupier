package svc

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/vault"
	"gorm.io/gorm"
)

// autoMigrate runs all necessary database migrations
func autoMigrate(db *gorm.DB) error {
	// Import all model packages and run their AutoMigrate functions
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate server models: %w", err)
	}

	if err := migrateAuxModels(db); err != nil {
		return err
	}

	// Migrate Agent Session model
	if err := reg.MigrateAgentSessions(db); err != nil {
		return fmt.Errorf("failed to migrate agent sessions: %w", err)
	}

	return nil
}

// migrateAuxModels bootstraps tables owned by packages that must not depend
// on internal/model (audit, registry metrics). It runs as part of the
// migration baseline only — constructors of these stores never run DDL.
func migrateAuxModels(db *gorm.DB) error {
	if err := db.AutoMigrate(&audit.AuditModel{}, &reg.AgentMetricsHistory{}); err != nil {
		return fmt.Errorf("failed to migrate aux models: %w", err)
	}
	return nil
}

// migrateMeta brings the meta database to the latest schema version via the
// versioned migration executor. The legacy AutoMigrate path only runs as the
// one-time baseline bridge on fresh/pre-versioning databases
// (docs/architecture/database-migration-strategy.md).
func migrateMeta(ctx context.Context, db *gorm.DB) (int64, error) {
	return migrate.EnsureUpToDate(ctx, db, migrate.ScopeMeta, func(db *gorm.DB) error {
		return autoMigrateMeta(db)
	})
}

// autoMigrateMeta migrates only meta-level models into the meta database
// (database-per-game architecture).
func autoMigrateMeta(db *gorm.DB) error {
	if err := model.AutoMigrateMeta(db); err != nil {
		return fmt.Errorf("failed to migrate meta models: %w", err)
	}
	if err := migrateAuxModels(db); err != nil {
		return err
	}
	if err := reg.MigrateAgentSessions(db); err != nil {
		return fmt.Errorf("failed to migrate agent sessions: %w", err)
	}
	return nil
}

// newGameRouter builds a database-per-game router wired to the resolved
// driver/DSN. The router auto-creates and migrates each game database on
// first use.
func newGameRouter(c config.Config, metaDB *gorm.DB, secrets vault.Resolver) *router.Router {
	driver, dsn, err := resolveDriverAndDSN(c, secrets)
	if err != nil {
		// 不可达论证（C 类）：metaDB 已用同一 resolver 展开同一 DSN 成功，
		// 此处重复解析无新增失败输入；err 为签名契约必须处理，保留 panic。
		panic(fmt.Sprintf("resolve game router DSN: %v", err))
	}
	dsnForDB := func(metaDSN, dbName string) string {
		return DSNForDatabase(driver, metaDSN, dbName)
	}
	cfg := router.Config{
		Driver:         driver,
		MetaDSN:        dsn,
		DSNForDatabase: dsnForDB,
		EnsureDatabase: EnsureGameDatabase,
		Open:           OpenGormForRouter,
		// ARCH-009: the router open budget and lifecycle context flow into the
		// game-scoped migration so a cancelled/budgeted first open aborts
		// rather than racing to finish.
		MigrateGame: func(ctx context.Context, db *gorm.DB) error {
			if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeGame, model.AutoMigrateGame); err != nil {
				return err
			}
			return nil
		},
	}
	if prefix := strings.TrimSpace(c.Database.GameDBPrefix); prefix != "" {
		cfg.NameForGame = func(gameID, env string) string {
			return prefix + sanitizeGameDBComponent(gameID) + "_" + sanitizeGameDBComponent(env)
		}
	}
	// ARCH-006: the authorized game_envs.database_name binding is the single
	// routing truth. GameDB opens by the binding name, never by recomputing
	// from (gameID, env) — so a custom binding name, a prefix change or an
	// underscore collision can no longer open the wrong or a duplicate database.
	// NameForGame remains only for creating/backfilling new bindings.
	cfg.ResolveDatabaseName = model.NewGameModel(metaDB).LookupDatabaseName
	return router.New(cfg, metaDB)
}

// sanitizeGameDBComponent lowercases and whitelists a game/env identifier for
// use inside a physical database name.
func sanitizeGameDBComponent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		s = "default"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// gameDBNameFor resolves the physical database name for a game scope using
// the configured prefix (mirrors the router wiring in newGameRouter).
func gameDBNameFor(c config.Config, gameID, env string) string {
	if prefix := strings.TrimSpace(c.Database.GameDBPrefix); prefix != "" {
		return prefix + sanitizeGameDBComponent(gameID) + "_" + sanitizeGameDBComponent(env)
	}
	return router.DefaultGameDBName(gameID, env)
}
