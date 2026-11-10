package router_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newScopeTestRouter builds a minimal sqlite-backed router (same shape as
// TestRouter_GameDB) for OpenScopes assertions.
func newScopeTestRouter(t *testing.T) (*router.Router, string) {
	t.Helper()
	tmp := t.TempDir()
	metaDBPath := filepath.Join(tmp, "meta.db")
	metaDB := openSQLite(t, metaDBPath)
	if err := model.AutoMigrateMeta(metaDB); err != nil {
		t.Fatalf("migrate meta: %v", err)
	}
	r := router.New(router.Config{
		Driver:  "sqlite",
		MetaDSN: metaDBPath,
		DSNForDatabase: func(metaDSN, dbName string) string {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db")
		},
		EnsureDatabase: func(_ context.Context, driver, metaDSN, dbName string) (string, error) {
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}, metaDB)
	t.Cleanup(func() {
		_ = r.Close()
		closeDB(t, metaDB)
	})
	return r, metaDBPath
}

func scopesContain(scopes []router.Scope, gameID, env string) bool {
	for _, sc := range scopes {
		if sc.GameID == gameID && sc.Env == env {
			return true
		}
	}
	return false
}

// TestRouter_OpenScopes lists exactly the opened scopes: never-opened scopes
// are absent (background sweeps must not trigger first-open side effects for
// dormant games), Forget removes a scope, and a fresh router reports none.
func TestRouter_OpenScopes(t *testing.T) {
	t.Parallel()

	r, _ := newScopeTestRouter(t)
	ctx := context.Background()

	if got := r.OpenScopes(); len(got) != 0 {
		t.Fatalf("fresh router OpenScopes = %v, want empty", got)
	}

	for _, sc := range [][2]string{{"demo", "prod"}, {"demo", "staging"}, {"rpg", "prod"}} {
		if _, err := r.GameDB(ctx, sc[0], sc[1]); err != nil {
			t.Fatalf("GameDB %s/%s: %v", sc[0], sc[1], err)
		}
	}
	got := r.OpenScopes()
	if len(got) != 3 {
		t.Fatalf("OpenScopes = %v, want 3 scopes", got)
	}
	for _, sc := range [][2]string{{"demo", "prod"}, {"demo", "staging"}, {"rpg", "prod"}} {
		if !scopesContain(got, sc[0], sc[1]) {
			t.Errorf("OpenScopes = %v, missing %s/%s", got, sc[0], sc[1])
		}
	}

	if err := r.Forget("demo", "staging"); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	got = r.OpenScopes()
	if len(got) != 2 || scopesContain(got, "demo", "staging") {
		t.Fatalf("OpenScopes after Forget = %v, want demo/staging dropped", got)
	}

	if err := r.ForgetGame("rpg"); err != nil {
		t.Fatalf("ForgetGame: %v", err)
	}
	got = r.OpenScopes()
	if len(got) != 1 || !scopesContain(got, "demo", "prod") {
		t.Fatalf("OpenScopes after ForgetGame = %v, want only demo/prod", got)
	}
}
