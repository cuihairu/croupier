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
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// DefaultFirstOpenTimeout bounds a single first-open (ensure+open+migrate) so a
// wedged database server cannot hold the shared singleflight forever
// (ARCH-009 budget). Config.FirstOpenTimeout overrides; <=0 uses this default.
const DefaultFirstOpenTimeout = 30 * time.Second

// DatabaseOpener opens a *gorm.DB for a given driver+DSN. Production wires
// this to the shared open logic in internal/svc; tests can inject fakes.
// The context is the router lifecycle context (see Router.ctx), so a shutdown
// interrupts an in-flight open; per-caller cancellation is honored at the wait.
type DatabaseOpener func(ctx context.Context, driver, dsn string) (*gorm.DB, error)

// gameDBInflightHook 是 GameDB singleflight 回调入口的测试接缝，产品代码
// 恒为 nil。唯一用途：让测试在「leader 赢得 Do 竞争」与「缓存 re-check」
// 之间同步注入竞争条目，确定性覆盖 re-check 分支（该窗口内产品代码自身
// 无挂起点，真实交错不可构造）。用后必须在 t.Cleanup 复位。
var gameDBInflightHook func()

// DatabaseEnsurer creates the physical database named dbName on the server
// reachable via baseDSN (which points at an existing/admin database), then
// returns the DSN to connect to the newly created database. If the database
// already exists it should be a no-op. The context is the router lifecycle
// context, so a shutdown interrupts a stalled ensure.
type DatabaseEnsurer func(ctx context.Context, driver, baseDSN, dbName string) (gameDSN string, err error)

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
	// The context is the router lifecycle context.
	MigrateGame func(ctx context.Context, db *gorm.DB) error
	// FirstOpenTimeout bounds a single first-open (ensure+open+migrate).
	// <=0 falls back to DefaultFirstOpenTimeout. ARCH-009 budget.
	FirstOpenTimeout time.Duration
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

	// lifecycleCtx/lifecycleCancel bound the shared first-open I/O
	// (ensure+open+migrate). It is cancelled by Close, so a shutdown
	// interrupts in-flight opens. It is NOT tied to any single caller's
	// context: first-open work is shared and must not be aborted because one
	// waiter disconnected — per-caller cancellation is honored at the wait
	// boundary (see openByPhysicalName). ARCH-009.
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	mu        sync.RWMutex
	cache     map[string]*gorm.DB // keyed by physical database name
	gameOfDB  map[string]string   // dbName → gameID (for ForgetGame)
	dbOfScope map[string]string   // scopeKey(gameID,env) → dbName (for Forget)

	// firstOpenLatency records the duration of the successful first-open per
	// physical database name. Exposed via FirstOpenLatency for the pool/首开
	// 耗时 observation surface (ARCH-009). Guarded by mu.
	firstOpenLatency map[string]time.Duration

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
	ctx, cancel := context.WithCancel(context.Background())
	return &Router{
		cfg:              cfg,
		metaDB:           metaDB,
		lifecycleCtx:     ctx,
		lifecycleCancel:  cancel,
		cache:            make(map[string]*gorm.DB),
		gameOfDB:         make(map[string]string),
		dbOfScope:        make(map[string]string),
		firstOpenLatency: make(map[string]time.Duration),
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
func (r *Router) openByPhysicalName(ctx context.Context, dbName, gameID, env string) (*gorm.DB, error) {
	// Fast path: read lock only, never blocks other scopes' opens.
	r.mu.RLock()
	if db, ok := r.cache[dbName]; ok {
		r.mu.RUnlock()
		return db, nil
	}
	r.mu.RUnlock()

	// Slow path: deduplicate per-db. The create/migrate/open I/O happens
	// inside openGameDB, outside any cache lock.
	ch := r.inflight.DoChan(dbName, func() (interface{}, error) {
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

		// 预算（ARCH-009）：单次首开（ensure+open+migrate）受 FirstOpenTimeout
		// 上限约束，避免数据库服务器卡死时 shared singleflight 永久悬挂。
		// 取消：I/O 消费路由生命周期 ctx（Close 时中断），而非任一调用方 ctx
		// ——首开是共享工作，不因单个等待者断连而中止。
		budget := r.cfg.FirstOpenTimeout
		if budget <= 0 {
			budget = DefaultFirstOpenTimeout
		}
		openCtx, cancel := context.WithTimeout(r.lifecycleCtx, budget)
		defer cancel()

		openStart := time.Now()
		db, err := r.openGameDB(openCtx, dbName)
		if err != nil {
			return nil, err
		}
		elapsed := time.Since(openStart)

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
		r.firstOpenLatency[dbName] = elapsed
		r.mu.Unlock()
		return db, nil
	})

	// 取消等待及时返回（ARCH-009）：等待首开完成期间调用方 ctx 被取消，
	// 立即返回 ctx.Err()，不阻塞到 leader 结束；leader 工作继续（结果入
	// 缓存供后续调用复用），不被本等待者取消打断。
	var res singleflight.Result
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res = <-ch:
	}
	if res.Err != nil {
		return nil, res.Err
	}
	return res.Val.(*gorm.DB), nil
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

// Close closes every cached game DB and cancels the lifecycle context so any
// in-flight first-open is interrupted. The meta DB is owned by the caller and
// is NOT closed here.
func (r *Router) Close() error {
	r.lifecycleCancel()
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

// PoolSize returns the number of currently cached open game databases
// (ARCH-009 exposure surface: 池数).
func (r *Router) PoolSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cache)
}

// FirstOpenLatency returns the duration of the successful first-open for the
// given physical database name (ARCH-009 exposure surface: 首开耗时).
// The second return is false when the database has never been first-opened by
// this router instance (cached entries seeded or opened before instrumentation).
func (r *Router) FirstOpenLatency(dbName string) (time.Duration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.firstOpenLatency[dbName]
	return d, ok
}

func (r *Router) openGameDB(ctx context.Context, dbName string) (*gorm.DB, error) {
	if r.cfg.DSNForDatabase == nil {
		return nil, fmt.Errorf("router: DSNForDatabase is not configured")
	}
	dsn := r.cfg.DSNForDatabase(r.cfg.MetaDSN, dbName)
	if r.cfg.EnsureDatabase != nil {
		var err error
		dsn, err = r.cfg.EnsureDatabase(ctx, r.cfg.Driver, r.cfg.MetaDSN, dbName)
		if err != nil {
			return nil, fmt.Errorf("router: ensure database %q: %w", dbName, err)
		}
	}
	db, err := r.cfg.Open(ctx, r.cfg.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("router: open database %q: %w", dbName, err)
	}
	if db == nil {
		return nil, fmt.Errorf("router: open database %q: opener returned nil", dbName)
	}
	// 连接就绪确认（ARCH-009）：把实际建连纳入 ctx 取消/预算——懒连接驱动
	// 的网络故障在此暴露并受 openCtx 约束，而非推迟到首条业务查询。
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.PingContext(ctx); err != nil {
			_ = closeQuietly(db)
			return nil, fmt.Errorf("router: ping database %q: %w", dbName, err)
		}
	}
	if r.cfg.MigrateGame != nil {
		if err := r.cfg.MigrateGame(ctx, db); err != nil {
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
