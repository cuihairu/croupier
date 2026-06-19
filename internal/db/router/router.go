// Package router implements the database-per-game connection management.
//
// The Router holds one *gorm.DB for the meta database (croupier_meta) and
// lazily opens one *gorm.DB per game database (e.g. game_demo_prod). Game
// databases are created on first use when missing.
//
// Routing is by logical key "gameID\x00env" which maps to a physical
// database name via a configurable naming function (default:
// "game_<gameID>_<env>"). The meta games/game_envs tables are the source of
// truth for which (gameID, env) pairs exist; the Router just manages
// connections.
package router

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// DatabaseOpener opens a *gorm.DB for a given driver+DSN. Production wires
// this to the shared open logic in internal/svc; tests can inject fakes.
type DatabaseOpener func(driver, dsn string) (*gorm.DB, error)

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

	mu       sync.RWMutex
	cache    map[string]*gorm.DB // keyed by physical database name
	gameOfDB map[string]string   // dbName → gameID (for ForgetGame)
}

// New constructs a Router. metaDB must already be open and migrated.
func New(cfg Config, metaDB *gorm.DB) *Router {
	if cfg.NameForGame == nil {
		cfg.NameForGame = DefaultGameDBName
	}
	return &Router{
		cfg:      cfg,
		metaDB:   metaDB,
		cache:    make(map[string]*gorm.DB),
		gameOfDB: make(map[string]string),
	}
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
// migrating the physical database on first use. Concurrent calls for the
// same scope are deduplicated by the cache.
func (r *Router) GameDB(_ context.Context, gameID, env string) (*gorm.DB, error) {
	dbName := r.cfg.NameForGame(gameID, env)
	if dbName == "" {
		return nil, fmt.Errorf("router: empty database name for game %q env %q", gameID, env)
	}

	r.mu.RLock()
	if db, ok := r.cache[dbName]; ok {
		r.mu.RUnlock()
		return db, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// Re-check under write lock.
	if db, ok := r.cache[dbName]; ok {
		return db, nil
	}

	db, err := r.openGameDB(dbName)
	if err != nil {
		return nil, err
	}
	r.cache[dbName] = db
	r.gameOfDB[dbName] = gameID
	return db, nil
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
	return firstErr
}

// Forget closes and removes the cached connection for a single (gameID, env)
// pair. Use this when an environment is deleted to avoid leaking connections.
// If the connection is not cached, it is a no-op. Subsequent GameDB calls for
// the same scope will re-open the database.
func (r *Router) Forget(gameID, env string) error {
	dbName := r.cfg.NameForGame(gameID, env)
	r.mu.Lock()
	defer r.mu.Unlock()
	db, ok := r.cache[dbName]
	if !ok {
		return nil
	}
	delete(r.cache, dbName)
	delete(r.gameOfDB, dbName)
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
	return firstErr
}

func (r *Router) openGameDB(dbName string) (*gorm.DB, error) {
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
