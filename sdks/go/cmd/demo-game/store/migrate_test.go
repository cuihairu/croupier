package store_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
)

// TestOpenRefusesPlatformDatabases is the catastrophic-mistake guard: DSNs
// pointing at the platform's databases must be refused before any connection
// or migration runs.
func TestOpenRefusesPlatformDatabases(t *testing.T) {
	cases := []string{
		"postgres://croupier:pw@localhost:5432/croupier?sslmode=disable",
		"postgresql://croupier:pw@localhost:5432/croupier",
		"postgres://croupier:pw@localhost:5432/croupier_meta",
		"croupier.db",
		"file:croupier.db?cache=shared",
		"/var/lib/croupier/data/croupier_meta.db",
		"host=localhost port=5432 user=croupier dbname=croupier password=pw",
	}
	for _, dsn := range cases {
		t.Run(dsn, func(t *testing.T) {
			backend, err := store.Open(dsn)
			if err == nil {
				_ = backend.Close()
				t.Fatal("platform DSN accepted")
			}
			if !strings.Contains(err.Error(), "platform database") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestOpenAcceptsDemoDatabase names: the guard must not over-refuse.
func TestOpenAcceptsDemoDatabase(t *testing.T) {
	if _, err := store.Open(filepath.Join(t.TempDir(), "demo_game.db")); err != nil {
		t.Fatalf("demo db refused: %v", err)
	}
	if _, err := store.Open(filepath.Join(t.TempDir(), "game_demo_prod.db")); err != nil {
		t.Fatalf("game db refused: %v", err)
	}
	if b, err := store.Open(""); err != nil {
		t.Fatalf("empty DSN (memory) refused: %v", err)
	} else {
		_ = b.Close()
	}
}

// TestDatabaseNameExtraction covers the DSN shape parsing behind the guard.
func TestDatabaseNameExtraction(t *testing.T) {
	cases := []struct {
		dsn  string
		want string
	}{
		{"postgres://u:p@h:5432/croupier?sslmode=disable", "croupier"},
		{"postgresql://u:p@h:5432/game_demo_prod", "game_demo_prod"},
		{"game_demo.db", "game_demo"},
		{":memory:", ""},
		{"file:/x/y/demo.db?mode=rwc", "demo"},
		{"host=h dbname=croupier", "croupier"},
	}
	for _, c := range cases {
		got, err := store.DatabaseName(c.dsn)
		if err != nil {
			t.Fatalf("%s: %v", c.dsn, err)
		}
		if got != c.want {
			t.Fatalf("%s: database name = %q, want %q", c.dsn, got, c.want)
		}
	}
}

// TestMigrateIdempotentReopen: AutoMigrate on an existing database (the
// every-boot path) must succeed without disturbing committed data.
func TestMigrateIdempotentReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "reopen.db")
	backend, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureSeed(1, store.DefaultSeedAnchor.UnixMilli(), false); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	backend2, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, err := store.NewStore("demo-game", "demo", backend2)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s2.Close()
	assertListTotal(t, s2, "players", 26)
	assertListTotal(t, s2, "orders", 30)
}
