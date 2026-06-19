package router_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openSQLite opens a SQLite database at the given path.
func openSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	return db
}

// closeDB closes the underlying sql.DB of a gorm.DB. Windows cannot delete
// SQLite files while a connection holds them, so tests must close before
// TempDir cleanup.
func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}

// TestDefaultGameDBName verifies the canonical database name derivation.
func TestDefaultGameDBName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		gameID, env, want string
	}{
		{"demo", "prod", "game_demo_prod"},
		{"demo", "staging", "game_demo_staging"},
		{"rpg", "prod", "game_rpg_prod"},
		{"Tower Defense", "Prod", "game_tower_defense_prod"},
		{"", "", "game_default_default"},
	}
	for _, tc := range cases {
		got := router.DefaultGameDBName(tc.gameID, tc.env)
		if got != tc.want {
			t.Errorf("DefaultGameDBName(%q,%q) = %q, want %q", tc.gameID, tc.env, got, tc.want)
		}
	}
}

// TestRouter_GameDB verifies that the router opens distinct databases for
// different (gameID, env) pairs and that game-scoped models route to the
// correct DB via dbctx.
func TestRouter_GameDB(t *testing.T) {
	t.Parallel()

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
		EnsureDatabase: func(driver, metaDSN, dbName string) (string, error) {
			// SQLite creates the file on open; just return the path.
			return filepath.Join(filepath.Dir(metaDSN), dbName+".db"), nil
		},
		Open: func(driver, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
		MigrateGame: func(db *gorm.DB) error {
			return model.AutoMigrateGame(db)
		},
	}, metaDB)
	defer func() {
		_ = r.Close()
		closeDB(t, metaDB)
	}()

	// Resolve two different game databases.
	dbA, err := r.GameDB(context.Background(), "demo", "prod")
	if err != nil {
		t.Fatalf("GameDB demo/prod: %v", err)
	}
	dbB, err := r.GameDB(context.Background(), "demo", "staging")
	if err != nil {
		t.Fatalf("GameDB demo/staging: %v", err)
	}

	// They must be different connections.
	if dbA == dbB {
		t.Fatal("expected different DB connections for prod vs staging")
	}

	// Insert a player into each DB; verify isolation.
	p1 := &model.Player{Username: "alice"}
	if err := dbA.Create(p1).Error; err != nil {
		t.Fatalf("create player in prod: %v", err)
	}
	p2 := &model.Player{Username: "bob"}
	if err := dbB.Create(p2).Error; err != nil {
		t.Fatalf("create player in staging: %v", err)
	}

	// prod DB should only have alice.
	var prodCount int64
	dbA.Model(&model.Player{}).Count(&prodCount)
	if prodCount != 1 {
		t.Errorf("prod player count = %d, want 1", prodCount)
	}

	// staging DB should only have bob.
	var stagingCount int64
	dbB.Model(&model.Player{}).Count(&stagingCount)
	if stagingCount != 1 {
		t.Errorf("staging player count = %d, want 1", stagingCount)
	}

	// Verify alice is NOT in staging.
	var alice model.Player
	result := dbB.Where("username = ?", "alice").First(&alice)
	if result.Error == nil {
		t.Error("alice should not exist in staging DB")
	}
}

// TestRouter_Caching verifies that repeated calls for the same scope return
// the cached connection.
func TestRouter_Caching(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	metaDBPath := filepath.Join(tmp, "meta.db")
	metaDB := openSQLite(t, metaDBPath)

	r := router.New(router.Config{
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
	}, metaDB)
	defer func() {
		_ = r.Close()
		closeDB(t, metaDB)
	}()

	db1, err := r.GameDB(context.Background(), "demo", "prod")
	if err != nil {
		t.Fatalf("first GameDB: %v", err)
	}
	db2, err := r.GameDB(context.Background(), "demo", "prod")
	if err != nil {
		t.Fatalf("second GameDB: %v", err)
	}
	if db1 != db2 {
		t.Error("expected cached DB connection to be reused")
	}
}

// TestRouter_DbctxResolution verifies that a game-scoped model picks up the
// correct DB from context.
func TestRouter_DbctxResolution(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	metaDBPath := filepath.Join(tmp, "meta.db")
	metaDB := openSQLite(t, metaDBPath)
	_ = model.AutoMigrateMeta(metaDB)

	r := router.New(router.Config{
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
		MigrateGame: func(db *gorm.DB) error {
			return model.AutoMigrateGame(db)
		},
	}, metaDB)
	defer func() {
		_ = r.Close()
		closeDB(t, metaDB)
	}()

	// Get the game DB and put it in context.
	gameDB, err := r.GameDB(context.Background(), "demo", "prod")
	if err != nil {
		t.Fatalf("GameDB: %v", err)
	}
	ctx := dbctx.WithDB(context.Background(), gameDB)

	// Create a player model bound to the meta DB (fallback).
	playerModel := model.NewPlayerModel(metaDB)

	// When the model resolves via dbctx, it should use the game DB, not meta.
	player := &model.Player{Username: "ctxplayer"}
	if err := playerModel.Create(ctx, player, ""); err != nil {
		t.Fatalf("create via context: %v", err)
	}

	// Verify the player exists in game DB.
	var count int64
	gameDB.Model(&model.Player{}).Where("username = ?", "ctxplayer").Count(&count)
	if count != 1 {
		t.Errorf("player not found in game DB, count=%d", count)
	}

	// Verify the player does NOT exist in meta DB. The meta DB does not have
	// a players table (only meta tables), so the query errors — that's the
	// expected confirmation.
	var metaCount int64
	metaErr := metaDB.Model(&model.Player{}).Where("username = ?", "ctxplayer").Count(&metaCount).Error
	if metaErr == nil && metaCount != 0 {
		t.Errorf("player should not be in meta DB, count=%d", metaCount)
	}
}
