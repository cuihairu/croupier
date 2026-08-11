package svc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
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
	if err := os.WriteFile(path, []byte(`{"games":[{"gameId":"demo","aliasName":"Demo"}]}`), 0o644); err != nil {
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

func TestSanitizeGameID_Extended(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		expect string
	}{
		{"single value", []string{"MyGame"}, "mygame"},
		{"with spaces", []string{"My Game"}, "my_game"},
		{"empty first", []string{"", "valid"}, "valid"},
		{"all empty", []string{"", ""}, ""},
		{"no args", []string{}, ""},
		{"trims whitespace", []string{"  Game  "}, "game"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeGameID(tt.values...)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestSanitizeAlias_Extended(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		expect string
	}{
		{"single value", []string{"My Game"}, "My Game"},
		{"empty first", []string{"", "valid"}, "valid"},
		{"all empty", []string{"", ""}, ""},
		{"no args", []string{}, ""},
		{"trims whitespace", []string{"  Game  "}, "Game"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeAlias(tt.values...)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestHumanizeGameID_Extended(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"with underscores", "my_game", "My Game"},
		{"with dashes", "my-game", "My Game"},
		{"single word", "game", "Game"},
		{"empty", "", ""},
		{"already capitalized", "My_Game", "My Game"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := humanizeGameID(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestPickGameColor_Extended(t *testing.T) {
	tests := []struct {
		name   string
		index  int
		expect string
	}{
		{"first", 0, fallbackGameColorCycle[0]},
		{"second", 1, fallbackGameColorCycle[1]},
		{"overflow", len(fallbackGameColorCycle), fallbackGameColorCycle[0]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pickGameColor(tt.index)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestPickEnvColor_Extended(t *testing.T) {
	defaults := map[string]model.GameEnv{
		"prod": {Color: "#FF0000"},
	}
	tests := []struct {
		name     string
		envName  string
		defaults map[string]model.GameEnv
		expect   string
	}{
		{"found in defaults", "prod", defaults, "#FF0000"},
		{"fallback name", "dev", defaults, fallbackEnvColors["dev"]},
		{"unknown name", "unknown", defaults, defaultGameColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pickEnvColor(tt.envName, tt.defaults)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestDefaultEnvColor_Extended(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"known", "dev", fallbackEnvColors["dev"]},
		{"unknown", "unknown", defaultGameColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultEnvColor(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestNormalizeColor_Extended(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		expect   string
	}{
		{"empty", "", "#000", "#000"},
		{"with hash", "#FF0000", "#000", "#ff0000"},
		{"without hash", "red", "#000", "red"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeColor(tt.value, tt.fallback)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestBoolPtr_Extended(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)
	assert.True(t, *truePtr)
	assert.False(t, *falsePtr)
}

func TestDefaultBootstrapGamesConfig_Extended(t *testing.T) {
	cfg := defaultBootstrapGamesConfig()
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DefaultEnvs)
	assert.Len(t, cfg.Games, 1)
	assert.Equal(t, "default", cfg.Games[0].GameID)
}

func TestDefaultBootstrapGame_Extended(t *testing.T) {
	envs := []model.GameEnv{
		{Env: "dev", Color: "#00FF00"},
		{Env: "prod", Color: "#FF0000"},
	}
	game := defaultBootstrapGame(envs)
	assert.Equal(t, "default", game.GameID)
	assert.Equal(t, "Default Game", game.AliasName)
	assert.Len(t, game.Envs, 2)
}

func TestResolveGamesConfigPath_Empty_Extended(t *testing.T) {
	cfg := config.Config{}
	result := resolveGamesConfigPath(cfg)
	assert.Equal(t, "", result)
}

func TestSanitizeGameEnvs_Extended(t *testing.T) {
	defaults := []model.GameEnv{
		{Env: "dev", Color: "#00FF00"},
		{Env: "prod", Color: "#FF0000"},
	}
	tests := []struct {
		name     string
		envs     []model.GameEnv
		defaults []model.GameEnv
		expect   int
	}{
		{"empty input", nil, defaults, 0},
		{"with input", []model.GameEnv{{Env: "dev"}}, defaults, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeGameEnvs(tt.envs, tt.defaults)
			assert.Len(t, result, tt.expect)
		})
	}
}

func TestEnsureSeedEnvs_Extended(t *testing.T) {
	tests := []struct {
		name     string
		entry    bootstrapGameSeedEntry
		defaults []model.GameEnv
		expect   int
	}{
		{
			name:     "with legacy env",
			entry:    bootstrapGameSeedEntry{Env: "prod"},
			defaults: fallbackDefaultEnvs,
			expect:   1,
		},
		{
			name:     "with envs",
			entry:    bootstrapGameSeedEntry{Envs: []model.GameEnv{{Env: "dev"}, {Env: "prod"}}},
			defaults: fallbackDefaultEnvs,
			expect:   2,
		},
		{
			name:     "with both legacy and envs",
			entry:    bootstrapGameSeedEntry{Env: "staging", Envs: []model.GameEnv{{Env: "dev"}}},
			defaults: fallbackDefaultEnvs,
			expect:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureSeedEnvs(tt.entry, tt.defaults)
			assert.Len(t, result, tt.expect)
		})
	}
}

func TestBuildGameFromSeed_Extended(t *testing.T) {
	tests := []struct {
		name    string
		entry   bootstrapGameSeedEntry
		wantErr bool
	}{
		{
			name: "valid entry",
			entry: bootstrapGameSeedEntry{
				GameID:    "test_game",
				AliasName: "Test Game",
				Status:    "dev",
				Enabled:   boolPtr(true),
			},
			wantErr: false,
		},
		{
			name:    "empty game id",
			entry:   bootstrapGameSeedEntry{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, err := buildGameFromSeed(tt.entry, fallbackDefaultEnvs, 0)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, game)
				assert.Equal(t, tt.entry.GameID, game.Name)
			}
		})
	}
}

func TestSanitizeEnvList_Extended(t *testing.T) {
	tests := []struct {
		name  string
		envs  []model.GameEnv
		count int
	}{
		{
			name:  "nil",
			envs:  nil,
			count: len(fallbackDefaultEnvs),
		},
		{
			name:  "empty",
			envs:  []model.GameEnv{},
			count: len(fallbackDefaultEnvs),
		},
		{
			name: "with duplicates",
			envs: []model.GameEnv{
				{Env: "dev", Color: "#00FF00"},
				{Env: "DEV", Color: "#FF0000"},
			},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEnvList(tt.envs)
			assert.Len(t, result, tt.count)
		})
	}
}
