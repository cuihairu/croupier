package games

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    repogames "github.com/cuihairu/croupier/internal/repo/gorm/games"
    dom "github.com/cuihairu/croupier/internal/ports"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

// newTestDB returns a sqlite in-memory DB.
func newTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("open sqlite: %v", err)
    }
    if err := repogames.AutoMigrate(db); err != nil {
        t.Fatalf("migrate: %v", err)
    }
    return db
}

func TestCreateGameAppliesDefaults(t *testing.T) {
    // Ensure configs/games.json is present (relative to repo root when running tests)
    if _, err := os.Stat(filepath.FromSlash("configs/games.json")); err != nil {
        t.Skip("configs/games.json not found; skip")
    }
    defs, err := LoadDefaultsFromFile(filepath.FromSlash("configs/games.json"))
    if err != nil {
        t.Fatalf("load defaults: %v", err)
    }
    db := newTestDB(t)
    repo := repogames.NewRepo(db)
    svc := NewService(repogames.NewPortRepo(repo), defs)

    // Create a game
    g := &dom.Game{Name: "test-game", Enabled: true}
    if err := svc.CreateGame(context.Background(), g); err != nil {
        t.Fatalf("create game: %v", err)
    }
    if g.ID == 0 { t.Fatalf("expected game ID assigned") }

    // Defaults should be added to its env list and env defs table
    got, err := svc.ListEnvRecords(context.Background(), g.ID)
    if err != nil {
        t.Fatalf("list env records: %v", err)
    }
    if len(got) != len(defs) {
        t.Fatalf("expected %d default envs, got %d", len(defs), len(got))
    }
}

func TestEnvCRUDViaService(t *testing.T) {
    db := newTestDB(t)
    repo := repogames.NewRepo(db)
    svc := NewService(repogames.NewPortRepo(repo), nil)
    // Create a game
    g := &dom.Game{Name: "g1", Enabled: true}
    if err := svc.CreateGame(context.Background(), g); err != nil { t.Fatal(err) }
    // Add env with meta
    if err := svc.AddEnv(context.Background(), g.ID, dom.GameEnvDef{Env: "prod", Description: "生产", Color: "#fff"}); err != nil { t.Fatal(err) }
    // Update env meta and rename
    if err := svc.UpdateEnv(context.Background(), g.ID, "prod", dom.GameEnvDef{Env: "production", Description: "生产环境", Color: "#000"}); err != nil { t.Fatal(err) }
    // Remove
    if err := svc.RemoveEnv(context.Background(), g.ID, "production"); err != nil { t.Fatal(err) }
    // Should be empty
    lst, err := svc.ListEnvRecords(context.Background(), g.ID)
    if err != nil { t.Fatal(err) }
    if len(lst) != 0 { t.Fatalf("expected 0 envs, got %d", len(lst)) }
}
