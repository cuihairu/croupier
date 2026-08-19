package router

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

func openSQLiteDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	return db
}

func newTestRouter(t *testing.T, cfg Config) (*Router, *gorm.DB) {
	t.Helper()
	tmp := t.TempDir()
	metaDBPath := filepath.Join(tmp, "meta.db")
	metaDB := openSQLiteDB(t, metaDBPath)
	base := Config{
		Driver:  "sqlite",
		MetaDSN: metaDBPath,
		DSNForDatabase: func(metaDSN, dbName string) string {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db")
		},
		EnsureDatabase: func(driver, metaDSN, dbName string) (string, error) {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}
	if cfg.NameForGame != nil {
		base.NameForGame = cfg.NameForGame
	}
	if cfg.EnsureDatabase != nil {
		base.EnsureDatabase = cfg.EnsureDatabase
	}
	if cfg.Open != nil {
		base.Open = cfg.Open
	}
	if cfg.MigrateGame != nil {
		base.MigrateGame = cfg.MigrateGame
	}
	t.Cleanup(func() {
		_ = openSQLiteClose(metaDB)
	})
	return New(base, metaDB), metaDB
}

func openSQLiteClose(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil
	}
	return sqlDB.Close()
}

func TestGameDB_EmptyDatabaseName(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		NameForGame: func(gameID, env string) string { return "" },
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("expected error for empty database name")
	}
	if got := err.Error(); !contains(got, "empty database name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGameDB_EnsureDatabaseError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		EnsureDatabase: func(driver, metaDSN, dbName string) (string, error) {
			return "", errors.New("ensure failed")
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("expected ensure database error")
	}
	if !contains(err.Error(), "ensure database") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGameDB_OpenError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return nil, errors.New("open failed")
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("expected open error")
	}
	if !contains(err.Error(), "open database") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGameDB_MigrateError(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		MigrateGame: func(db *gorm.DB) error {
			return errors.New("migrate failed")
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("expected migrate error")
	}
	if !contains(err.Error(), "migrate database") {
		t.Fatalf("unexpected error: %v", err)
	}
	// A failed migration must not leave a cached entry.
	r.mu.RLock()
	cached := len(r.cache)
	r.mu.RUnlock()
	if cached != 0 {
		t.Fatalf("expected empty cache after migration failure, got %d", cached)
	}
}

func TestGameDB_ConcurrentSameScope(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	const workers = 8
	dbs := make([]*gorm.DB, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			db, err := r.GameDB(context.Background(), "demo", "prod")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			dbs[idx] = db
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("concurrent GameDB: %v", firstErr)
	}
	for i := 1; i < workers; i++ {
		if dbs[i] == nil || dbs[0] == nil {
			t.Fatalf("worker %d got nil db", i)
		}
		if dbs[i] != dbs[0] {
			t.Fatalf("worker %d got a different connection; cache is not shared", i)
		}
	}
}

func TestResolve_ErrorPath(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		EnsureDatabase: func(driver, metaDSN, dbName string) (string, error) {
			return "", errors.New("ensure failed")
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	ctx, db, err := r.Resolve(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("expected error from Resolve")
	}
	if db != nil {
		t.Fatal("expected nil DB on error")
	}
	if ctx == nil {
		t.Fatal("context must never be nil")
	}
}

func TestClose_DummyDialectorDB(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	// A gorm.DB not backed by *sql.DB makes db.DB() fail; Close must skip it
	// gracefully and still clear the cache.
	dummy, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open dummy: %v", err)
	}
	r.cache["game_dummy_prod"] = dummy
	r.gameOfDB["game_dummy_prod"] = "dummy"

	if err := r.Close(); err != nil {
		t.Fatalf("Close should swallow db.DB() errors, got %v", err)
	}
	if len(r.cache) != 0 || len(r.gameOfDB) != 0 {
		t.Fatal("Close must clear the cache")
	}
}

func TestForget_NotCachedNoop(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	if err := r.Forget("missing", "prod"); err != nil {
		t.Fatalf("Forget on uncached scope should be a no-op, got %v", err)
	}
}

func TestForget_DummyDialectorDB(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	dummy, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open dummy: %v", err)
	}
	r.cache["game_dummy_prod"] = dummy
	r.gameOfDB["game_dummy_prod"] = "dummy"

	if err := r.Forget("dummy", "prod"); err != nil {
		t.Fatalf("Forget should swallow db.DB() errors, got %v", err)
	}
	if _, ok := r.cache["game_dummy_prod"]; ok {
		t.Fatal("Forget must drop the cached entry")
	}
}

func TestForgetGame_SkipsOtherGames(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	dbA, err := r.GameDB(context.Background(), "gamea", "prod")
	if err != nil {
		t.Fatalf("GameDB gamea: %v", err)
	}
	dbB, err := r.GameDB(context.Background(), "gameb", "prod")
	if err != nil {
		t.Fatalf("GameDB gameb: %v", err)
	}

	if err := r.ForgetGame("gamea"); err != nil {
		t.Fatalf("ForgetGame: %v", err)
	}
	if _, ok := r.cache["game_gamea_prod"]; ok {
		t.Fatal("gamea entry should be removed")
	}
	if _, ok := r.cache["game_gameb_prod"]; !ok {
		t.Fatal("gameb entry must survive ForgetGame(gamea)")
	}
	if dbA == nil || dbB == nil {
		t.Fatal("dbs must not be nil")
	}
}

func TestForgetGame_DummyDialectorDB(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{})
	defer func() { _ = openSQLiteClose(metaDB) }()

	dummy, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open dummy: %v", err)
	}
	r.cache["game_dummy_prod"] = dummy
	r.gameOfDB["game_dummy_prod"] = "dummy"

	if err := r.ForgetGame("dummy"); err != nil {
		t.Fatalf("ForgetGame should swallow db.DB() errors, got %v", err)
	}
	if len(r.cache) != 0 {
		t.Fatal("ForgetGame must drop matching entries")
	}
}

func TestCloseQuietly_Nil(t *testing.T) {
	if err := closeQuietly(nil); err != nil {
		t.Fatalf("closeQuietly(nil) = %v, want nil", err)
	}
}

func TestCloseQuietly_DummyDialectorDB(t *testing.T) {
	dummy, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open dummy: %v", err)
	}
	if err := closeQuietly(dummy); err == nil {
		t.Fatal("closeQuietly on non-sql.DB should return the db.DB() error")
	}
}

func TestCloseQuietly_RealDB(t *testing.T) {
	tmp := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tmp, "real.db"))
	if err := closeQuietly(db); err != nil {
		t.Fatalf("closeQuietly real db: %v", err)
	}
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }
