package router

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// bindingResolver returns a ResolveDatabaseName backed by an in-memory map —
// a fake of the authorized game_envs binding. A missing key means "absent
// binding" and yields "" so the router must error rather than recompute.
func bindingResolver(bindings map[string]string) func(context.Context, string, string) (string, error) {
	return func(_ context.Context, gameID, env string) (string, error) {
		if v, ok := bindings[gameID+"\x00"+env]; ok {
			return v, nil
		}
		return "", nil
	}
}

// TestGameDB_UsesAuthorizedBindingName: custom name/legacy name/prefix change —
// when the binding name differs from the NameForGame recompute result (custom
// physical name, legacy prefix, or a recomputed name after a prefix change),
// routing must follow the authorized binding.database_name and never recompute
// to open a database.
func TestGameDB_UsesAuthorizedBindingName(t *testing.T) {
	custom := "custom_physical_db"
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(map[string]string{
			"demo\x00prod": custom, // entirely different from DefaultGameDBName("demo","prod")="game_demo_prod"
		}),
		NameForGame: DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	var opened string
	r.cfg.EnsureDatabase = func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
		opened = dbName
		return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
	}

	db, err := r.GameDB(context.Background(), "demo", "prod")
	if err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	if db == nil {
		t.Fatal("GameDB returned nil")
	}
	if opened != custom {
		t.Fatalf("opened db = %q, want authorized binding name %q (no recompute)", opened, custom)
	}
	if DefaultGameDBName("demo", "prod") == custom {
		t.Fatal("fixture must differ from the recomputed name to prove binding wins")
	}
}

// TestGameDB_MissingBindingErrors: missing binding — must not silently recompute
// and open a database (ARCH-006 removes prefix recomputation). An absent binding
// is a routing error; Open/Ensure never run.
func TestGameDB_MissingBindingErrors(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(map[string]string{}),
		NameForGame:         DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	var openCalls atomic.Int32
	r.cfg.Open = func(driver, dsn string) (*gorm.DB, error) {
		openCalls.Add(1)
		return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	}

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("GameDB on absent binding must error, not silently recompute")
	}
	if openCalls.Load() != 0 {
		t.Fatalf("no database must be opened on absent binding, got %d opens", openCalls.Load())
	}
}

// TestGameDB_LegacyBindingRoutes: legacy name — a pre-existing binding stores
// the canonical name recomputed at the time ("game_demo_prod"); after switching
// to binding-as-truth, still open by that name — no data migration, no renamed
// database created.
func TestGameDB_LegacyBindingRoutes(t *testing.T) {
	legacy := DefaultGameDBName("demo", "prod") // "game_demo_prod"
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(map[string]string{
			"demo\x00prod": legacy,
		}),
		NameForGame: DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	var opened string
	r.cfg.EnsureDatabase = func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
		opened = dbName
		return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
	}
	if _, err := r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	if opened != legacy {
		t.Fatalf("legacy binding must keep routing to %q, got %q", legacy, opened)
	}
}

// TestGameDB_UnderscoreCollisionDisambiguated: underscore collision — two scopes
// recompute to the same name ("game_a_b_c") but have distinct binding names;
// routing by binding truth opens each scope's own database, with no key
// collision and never the wrong database.
func TestGameDB_UnderscoreCollisionDisambiguated(t *testing.T) {
	// The two scopes' DefaultGameDBName collide (both sanitize to game_a_b_c).
	if DefaultGameDBName("a_b", "c") != DefaultGameDBName("a", "b_c") {
		t.Fatal("fixture requires colliding recomputed names")
	}
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(map[string]string{
			"a_b\x00c": "db_abc_scope1",
			"a\x00b_c": "db_abc_scope2",
		}),
		NameForGame: DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	var mu sync.Mutex
	var opened []string
	r.cfg.EnsureDatabase = func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
		mu.Lock()
		opened = append(opened, dbName)
		mu.Unlock()
		return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
	}

	if _, err := r.GameDB(context.Background(), "a_b", "c"); err != nil {
		t.Fatalf("GameDB a_b/c: %v", err)
	}
	if _, err := r.GameDB(context.Background(), "a", "b_c"); err != nil {
		t.Fatalf("GameDB a/b_c: %v", err)
	}
	if len(opened) != 2 || opened[0] == opened[1] {
		t.Fatalf("colliding scopes must open distinct binding-named DBs, got %v", opened)
	}
	if opened[0] != "db_abc_scope1" || opened[1] != "db_abc_scope2" {
		t.Fatalf("must open by binding names, got %v", opened)
	}
}

// TestGameDB_ConcurrentFirstOpenSingleflight: concurrent first open — concurrent
// GameDB calls for the same scope still open the database exactly once on the
// binding-truth path (singleflight dedup).
func TestGameDB_ConcurrentFirstOpenSingleflight(t *testing.T) {
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(map[string]string{
			"demo\x00prod": "game_demo_prod",
		}),
		NameForGame: DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	var openCalls atomic.Int32
	r.cfg.Open = func(driver, dsn string) (*gorm.DB, error) {
		openCalls.Add(1)
		return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := r.GameDB(context.Background(), "demo", "prod")
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if got := openCalls.Load(); got != 1 {
		t.Fatalf("concurrent first open must open once, got %d", got)
	}
}

// TestGameDB_ResolveErrorPropagates: when the binding lookup itself fails (meta DB
// failure), the error propagates with context instead of silently degrading to
// recomputation.
func TestGameDB_ResolveErrorPropagates(t *testing.T) {
	boom := errors.New("meta db down")
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: func(_ context.Context, _, _ string) (string, error) {
			return "", boom
		},
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	_, err := r.GameDB(context.Background(), "demo", "prod")
	if err == nil {
		t.Fatal("resolve error must propagate")
	}
}

// TestForget_AfterBindingDeleted: EnvDelete deletes the game_envs row first,
// then calls Forget — with the binding already gone, Forget must still locate and
// close the connection by the remembered scope (it cannot rely on a recomputed
// name, otherwise connections leak or the wrong one closes).
func TestForget_AfterBindingDeleted(t *testing.T) {
	bindings := map[string]string{"demo\x00prod": "custom_db_name"}
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(bindings),
		NameForGame:         DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	if _, err := r.GameDB(context.Background(), "demo", "prod"); err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	// Simulate the EnvDelete order: the binding row is deleted first.
	delete(bindings, "demo\x00prod")

	if err := r.Forget("demo", "prod"); err != nil {
		t.Fatalf("Forget after binding deletion: %v", err)
	}
	if len(r.cache) != 0 {
		t.Fatalf("Forget must drop the cached connection even without a binding, cache=%d", len(r.cache))
	}
}

// TestForgetGame_ByTrackedScope: ForgetGame matches on the cached gameID,
// independent of whether the binding still exists.
func TestForgetGame_ByTrackedScope(t *testing.T) {
	bindings := map[string]string{
		"demo\x00prod":  "game_demo_prod",
		"demo\x00dev":   "game_demo_dev",
		"other\x00prod": "game_other_prod",
	}
	r, metaDB := newTestRouter(t, Config{
		ResolveDatabaseName: bindingResolver(bindings),
		NameForGame:         DefaultGameDBName,
	})
	defer func() { _ = openSQLiteClose(metaDB) }()

	for _, sc := range [][2]string{{"demo", "prod"}, {"demo", "dev"}, {"other", "prod"}} {
		if _, err := r.GameDB(context.Background(), sc[0], sc[1]); err != nil {
			t.Fatalf("GameDB %v: %v", sc, err)
		}
	}
	if err := r.ForgetGame("demo"); err != nil {
		t.Fatalf("ForgetGame: %v", err)
	}
	if len(r.cache) != 1 {
		t.Fatalf("only other/prod must remain after ForgetGame(demo), cache=%d", len(r.cache))
	}
	if _, ok := r.cache["game_other_prod"]; !ok {
		t.Fatal("other/prod connection must survive")
	}
}
