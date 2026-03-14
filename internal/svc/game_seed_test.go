package svc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
)

func TestSanitizeEnvListFallsBackToDefaults(t *testing.T) {
	t.Parallel()

	got := sanitizeEnvList(nil)
	if len(got) != len(fallbackDefaultEnvs) {
		t.Fatalf("expected %d envs, got %d", len(fallbackDefaultEnvs), len(got))
	}
}

func TestSanitizeEnvListDeduplicatesAndNormalizesColor(t *testing.T) {
	t.Parallel()

	got := sanitizeEnvList([]model.GameEnv{
		{Env: "prod", Color: "#ABCDEF"},
		{Env: "PROD", Color: "#123456"},
		{Env: "dev"},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 envs, got %d", len(got))
	}
	if got[0].Color != "#abcdef" {
		t.Fatalf("expected normalized color, got %q", got[0].Color)
	}
	if got[1].Color == "" {
		t.Fatal("expected fallback color for dev")
	}
}

func TestBuildGameFromSeedUsesDefaults(t *testing.T) {
	t.Parallel()

	game, err := buildGameFromSeed(bootstrapGameSeedEntry{
		GameID:    "tower_defense",
		AliasName: "Tower Defense",
	}, fallbackDefaultEnvs, 1)
	if err != nil {
		t.Fatalf("buildGameFromSeed() error = %v", err)
	}
	if game.Name != "tower_defense" {
		t.Fatalf("expected game id, got %q", game.Name)
	}
	if game.AliasName != "Tower Defense" {
		t.Fatalf("expected alias name, got %q", game.AliasName)
	}
	if game.Color == "" {
		t.Fatal("expected fallback color")
	}
	envs, err := game.GetEnvs()
	if err != nil {
		t.Fatalf("GetEnvs() error = %v", err)
	}
	if len(envs) != len(fallbackDefaultEnvs) {
		t.Fatalf("expected default envs, got %d", len(envs))
	}
}

func TestBuildGameFromSeedRequiresGameID(t *testing.T) {
	t.Parallel()

	if _, err := buildGameFromSeed(bootstrapGameSeedEntry{}, fallbackDefaultEnvs, 0); err == nil {
		t.Fatal("expected error when game id missing")
	}
}

func TestEnsureSeedEnvsSupportsLegacyEnv(t *testing.T) {
	t.Parallel()

	envs := ensureSeedEnvs(bootstrapGameSeedEntry{Env: "prod"}, fallbackDefaultEnvs)
	if len(envs) != 1 || envs[0].Env != "prod" {
		t.Fatalf("unexpected envs: %#v", envs)
	}
}

func TestSanitizeHelpers(t *testing.T) {
	t.Parallel()

	if got := sanitizeGameID(" Tower Defense "); got != "tower_defense" {
		t.Fatalf("unexpected sanitizeGameID result: %q", got)
	}
	if got := sanitizeAlias("", " Display "); got != "Display" {
		t.Fatalf("unexpected sanitizeAlias result: %q", got)
	}
	if got := humanizeGameID("tower_defense-game"); got != "Tower Defense Game" {
		t.Fatalf("unexpected humanizeGameID result: %q", got)
	}
}

func TestColorHelpers(t *testing.T) {
	t.Parallel()

	if got := pickGameColor(len(fallbackGameColorCycle) + 1); got == "" {
		t.Fatal("expected cyclic game color")
	}
	if got := pickEnvColor("prod", map[string]model.GameEnv{}); got != "#13c2c2" {
		t.Fatalf("unexpected prod env color: %q", got)
	}
	if got := normalizeColor("#ABCDEF", "#000"); got != "#abcdef" {
		t.Fatalf("unexpected normalized color: %q", got)
	}
	if got := normalizeColor("", "#000"); got != "#000" {
		t.Fatalf("unexpected fallback color: %q", got)
	}
}

func TestLoadGamesBootstrapConfigAndResolvePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "games.json")
	if err := os.WriteFile(path, []byte(`{"games":[{"game_id":"demo","alias_name":"Demo"}]}`), 0o644); err != nil {
		t.Fatalf("write games config failed: %v", err)
	}

	cfg := config.Config{}
	cfg.Auth.GamesConfig = path
	resolved := resolveGamesConfigPath(cfg)
	if resolved == "" {
		t.Fatal("expected config path to resolve")
	}

	loaded, err := loadGamesBootstrapConfig(cfg)
	if err != nil {
		t.Fatalf("loadGamesBootstrapConfig() error = %v", err)
	}
	if len(loaded.Games) != 1 || loaded.Games[0].GameID != "demo" {
		t.Fatalf("unexpected loaded games: %#v", loaded.Games)
	}
}
