// Package router implements the database-per-game connection management.
//
// The Router holds one *gorm.DB for the meta database (croupier_meta) and
// lazily opens one *gorm.DB per game database (e.g. game_demo_prod). Game
// databases are created on first use when missing.
//
// Routing is by logical key "gameID\x00env". The physical database name for a
// scope comes from the meta database's game_envs binding (database_name) —
// the single source of truth, authorized at binding creation. The configurable
// naming function (default: "game_<gameID>_<env>") is only a fallback for
// scopes that have no binding yet (and the default for NEW bindings). The meta
// games/game_envs tables are the source of truth for which (gameID, env) pairs
// exist; the Router just manages connections.
package router

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// DatabaseOpener opens a *gorm.DB for a given driver+DSN. Production wires
// this to the shared open logic in internal/svc; tests can inject fakes.
type DatabaseOpener func(driver, dsn string) (*gorm.DB, error)

// gameDBInflightHook 是 GameDB singleflight 回调入口的测试接缝，产品代码
// 恒为 nil。唯一用途：让测试在「leader 赢得 Do 竞争」与「缓存 re-check」
// 之间同步注入竞争条目，确定性覆盖 re-check 分支（该窗口内产品代码自身
// 无挂起点，真实交错不可构造）。用后必须在 t.Cleanup 复位。
var gameDBInflightHook func()

// DatabaseEnsurer creates the physical database named dbName on the server
// reachable via baseDSN (which points at an existing/admin database), then
// returns the DSN to connect to the newly created database. If the database
// already exists it should be a no-op.
type DatabaseEnsurer func(driver, baseDSN, dbName string) (gameDSN string, err error)

// Config controls Router behavior.
type Config struct {
	// Driver is the DB driver shared by meta and game DBs (sqlite, postgres,
	// mysql, sqlserver).
	Driver string
	// MetaDSN is the DSN of the meta database (already open as MetaDB).
	MetaDSN string
	// NameForGame maps (gameID, env) to a physical database name. When nil,
	// DefaultGameDBName is used.
	NameForGame func(gameID, env string) string
	// DSNForDatabase returns a DSN that points at the given database name,
	// derived from the meta DSN. Required for non-sqlite drivers.
	// For sqlite, it should return the file path for the given db name.
	DSNForDatabase func(metaDSN, dbName string) string
	// EnsureDatabase creates the physical database if missing and returns the
	// DSN to connect to it. Required. For sqlite this is a no-op (file is
	// auto-created on open).
	EnsureDatabase DatabaseEnsurer
	// Open opens a *gorm.DB for a DSN. Required.
	Open DatabaseOpener
	// MigrateGame runs game-scoped migrations against a freshly opened game
	// DB. Optional; when set it runs once per game DB right after opening.
	MigrateGame func(db *gorm.DB) error
}

// DefaultGameDBName produces the canonical database name for a game scope:
// "game_<gameID>_<env>". Characters in gameID/env that are not [a-z0-9_-] are
// replaced with "_" to keep database identifiers safe.
func DefaultGameDBName(gameID, env string) string {
	sanitize := func(s string) string {
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
	return fmt.Sprintf("game_%s_%s", sanitize(gameID), sanitize(env))
}

// Router manages the meta DB and a cache of per-game DBs.
type Router struct {
	cfg    Config
	metaDB *gorm.DB

	mu        sync.RWMutex
	cache     map[string]*gorm.DB // keyed by physical database name
	gameOfDB  map[string]string   // dbName → gameID (for ForgetGame)
	dbOfScope map[string]string   // scopeKey(gameID,env) → dbName (for Forget)

	// inflight deduplicates concurrent first-open attempts per physical
	// database name so that expensive I/O (create/migrate) runs once per db
	// while different dbs open in parallel. I/O happens OUTSIDE the cache
	// mutex to avoid blocking readers on other scopes.
	inflight singleflight.Group
}

// New constructs a Router. metaDB must already be open and migrated.
func New(cfg Config, metaDB *gorm.DB) *Router {
	if cfg.NameForGame == nil {
		cfg.NameForGame = DefaultGameDBName
	}
	return &Router{
		cfg:       cfg,
		metaDB:    metaDB,
		cache:     make(map[string]*gorm.DB),
		gameOfDB:  make(map[string]string),
		dbOfScope: make(map[string]string),
	}
}

// scopeKey is the canonical logical key for a (gameID, env) pair. It is used
// only for map lookups (never a database value), so the NUL separator is safe.
func scopeKey(gameID, env string) string { return gameID + "\x00" + env }

// authoritativeName returns the physical database name for a game scope.
//
// ARCH-006: the meta database's game_envs binding is the single source of
// truth. Its database_name is the value authorized at binding creation and may
// legitimately differ from the prefix convention (custom names, a changed
// prefix, or disambiguating underscore collisions). When no binding exists or
// the meta DB cannot be consulted, it falls back to the configured naming
// function — that fallback only ever applies while a binding is missing (e.g.
// pre-backfill), never to override an existing binding.
func (r *Router) authoritativeName(ctx context.Context, gameID, env string) string {
	if r.metaDB != nil {
		var row struct {
			DatabaseName string
		}
		// Explicit deleted_at IS NULL: anonymous row struct does not carry a
		// gorm.DeletedAt, so gorm would not add the soft-delete clause itself.
		// Mirrors LookupDatabaseName (First on the model) semantics: a
		// soft-deleted binding is not routable.
		err := r.metaDB.WithContext(ctx).Table("game_envs").
			Select("database_name").
			Where("game_id = ? AND env = ? AND deleted_at IS NULL", gameID, env).
			Take(&row).Error
		if err == nil && strings.TrimSpace(row.DatabaseName) != "" {
			return strings.TrimSpace(row.DatabaseName)
		}
	}
	return r.cfg.NameForGame(gameID, env)
}

// MetaDB returns the meta database connection.
func (r *Router) MetaDB() *gorm.DB { return r.metaDB }

// NameForGame returns the physical database name the router would use for the
// given (gameID, env). This is the public accessor that mirrors the internal
// naming function, allowing API services to persist the same name into
// GameEnvBinding records.
func (r *Router) NameForGame(gameID, env string) string {
	return r.cfg.NameForGame(gameID, env)
}

// GameDB returns the *gorm.DB for the given (gameID, env), opening and
// migrating the physical database on first use. The physical database name is
// the meta binding's authoritative database_name (see authoritativeName),
// falling back to the configured naming function only when no binding exists.
// Concurrent calls for the same scope are deduplicated by singleflight;
// different scopes open in parallel. All I/O (create/migrate/open) runs
// OUTSIDE the cache mutex so a slow first-open for one game never blocks reads
// on another game's cached connection.
func (r *Router) GameDB(ctx context.Context, gameID, env string) (*gorm.DB, error) {
	dbName := r.authoritativeName(ctx, gameID, env)
	if dbName == "" {
		return nil, fmt.Errorf("router: empty database name for game %q env %q", gameID, env)
	}
	return r.openByPhysicalName(ctx, dbName, gameID, env)
}

// GameDBByPhysicalName returns the *gorm.DB for an already-authorized physical
// database name, skipping the binding lookup. Callers that resolved the name
// from the meta binding (e.g. the request middleware, which already fetched it
// for scope validation) use this to avoid a duplicate lookup. The name must be
// non-empty and previously authorized; no default naming is applied.
func (r *Router) GameDBByPhysicalName(ctx context.Context, dbName, gameID, env string) (*gorm.DB, error) {
	if strings.TrimSpace(dbName) == "" {
		return nil, fmt.Errorf("router: empty physical database name for game %q env %q", gameID, env)
	}
	return r.openByPhysicalName(ctx, strings.TrimSpace(dbName), gameID, env)
}

// openByPhysicalName is the shared fast/slow open path keyed by physical
// database name. The name has already been resolved (either from the meta
// binding or the configured naming function) by the caller.
func (r *Router) openByPhysicalName(_ context.Context, dbName, gameID, env string) (*gorm.DB, error) {
	// Fast path: read lock only, never blocks other scopes' opens.
	r.mu.RLock()
	if db, ok := r.cache[dbName]; ok {
		r.mu.RUnlock()
		return db, nil
	}
	r.mu.RUnlock()

	// Slow path: deduplicate per-db. The create/migrate/open I/O happens
	// inside openGameDB, outside any cache lock.
	v, err, _ := r.inflight.Do(dbName, func() (interface{}, error) {
		// 测试接缝（产品恒 nil）：在回调入口同步注入「等待窗口内他人抢先
		// 发布」的交错，使下方 re-check 分支可确定性覆盖——替代已退役的
		// 锁泊车/TryLock 自旋相位对齐编排（时序分支用注入点）。
		if gameDBInflightHook != nil {
			gameDBInflightHook()
		}
		// Re-check under read lock once we win the race: another goroutine may
		// have populated the cache while we were waiting.
		r.mu.RLock()
		if db, ok := r.cache[dbName]; ok {
			r.mu.RUnlock()
			return db, nil
		}
		r.mu.RUnlock()

		db, err := r.openGameDB(dbName)
		if err != nil {
			return nil, err
		}

		// Publish under a brief write lock. Only map writes happen here, no I/O.
		r.mu.Lock()
		// Another opener for the same db could have raced ahead (e.g. after a
		// Forget); prefer the existing one and close ours to avoid a leak.
		if existing, ok := r.cache[dbName]; ok {
			r.mu.Unlock()
			_ = closeQuietly(db)
			return existing, nil
		}
		r.cache[dbName] = db
		r.gameOfDB[dbName] = gameID
		r.dbOfScope[scopeKey(gameID, env)] = dbName
		r.mu.Unlock()
		return db, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*gorm.DB), nil
}

// Resolve is a convenience that returns the game DB and stores it in ctx via
// dbctx.WithDB, so downstream model calls pick it up automatically.
func (r *Router) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	db, err := r.GameDB(ctx, gameID, env)
	if err != nil {
		return ctx, nil, err
	}
	return dbctx.WithDB(ctx, db), db, nil
}

// Close closes every cached game DB. The meta DB is owned by the caller and
// is NOT closed here.
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for name, db := range r.cache {
		if sqlDB, err := db.DB(); err == nil {
			if err := sqlDB.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		delete(r.cache, name)
		delete(r.gameOfDB, name)
	}
	r.dbOfScope = make(map[string]string)
	return firstErr
}

// Forget closes and removes the cached connection for a single (gameID, env)
// pair. Use this when an environment is deleted to avoid leaking connections.
// If the connection is not cached, it is a no-op. Subsequent GameDB calls for
// the same scope will re-open the database.
//
// The connection is located by the scope→name mapping recorded at publish
// time, so it works regardless of how the name was derived (meta binding vs
// configured naming function) and survives binding deletions.
func (r *Router) Forget(gameID, env string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(gameID, env)
	dbName, ok := r.dbOfScope[key]
	if !ok {
		// Not published via the scope→name map (e.g. a connection seeded
		// directly, or from a path that predates the map): fall back to the
		// configured name so Forget still finds it.
		dbName = r.cfg.NameForGame(gameID, env)
	}
	db, ok := r.cache[dbName]
	if !ok {
		delete(r.dbOfScope, key)
		return nil
	}
	delete(r.cache, dbName)
	delete(r.gameOfDB, dbName)
	delete(r.dbOfScope, key)
	sqlDB, err := db.DB()
	if err != nil {
		return nil
	}
	return sqlDB.Close()
}

// ForgetGame closes and removes all cached connections belonging to the given
// gameID, regardless of env. Use this when an entire game is deleted to clean
// up all its environment connections.
func (r *Router) ForgetGame(gameID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for name, gdb := range r.cache {
		if r.gameOfDB[name] != gameID {
			continue
		}
		delete(r.cache, name)
		delete(r.gameOfDB, name)
		if sqlDB, err := gdb.DB(); err == nil {
			if err := sqlDB.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	// Prune scope mappings whose game no longer has a cached connection.
	for key, dbName := range r.dbOfScope {
		if r.gameOfDB[dbName] == "" {
			delete(r.dbOfScope, key)
		}
	}
	return firstErr
}

func (r *Router) openGameDB(dbName string) (*gorm.DB, error) {
	if r.cfg.DSNForDatabase == nil {
		return nil, fmt.Errorf("router: DSNForDatabase is not configured")
	}
	dsn := r.cfg.DSNForDatabase(r.cfg.MetaDSN, dbName)
	if r.cfg.EnsureDatabase != nil {
		var err error
		dsn, err = r.cfg.EnsureDatabase(r.cfg.Driver, r.cfg.MetaDSN, dbName)
		if err != nil {
			return nil, fmt.Errorf("router: ensure database %q: %w", dbName, err)
		}
	}
	db, err := r.cfg.Open(r.cfg.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("router: open database %q: %w", dbName, err)
	}
	if r.cfg.MigrateGame != nil {
		if err := r.cfg.MigrateGame(db); err != nil {
			_ = closeQuietly(db)
			return nil, fmt.Errorf("router: migrate database %q: %w", dbName, err)
		}
	}
	return db, nil
}

func closeQuietly(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
