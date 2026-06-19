package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
)

var (
	defaultGameColor       = "#8c8c8c"
	fallbackGameColorCycle = []string{
		"#8c8c8c", "#1677ff", "#13c2c2", "#722ed1", "#eb2f96", "#fa8c16", "#52c41a", "#a0d911",
	}
	fallbackEnvColors = map[string]string{
		"prod":        "#13c2c2",
		"production":  "#13c2c2",
		"stage":       "#fa8c16",
		"staging":     "#fa8c16",
		"test":        "#722ed1",
		"testing":     "#722ed1",
		"qa":          "#722ed1",
		"dev":         "#1677ff",
		"development": "#1677ff",
		"sandbox":     "#2f54eb",
	}
)

var fallbackDefaultEnvs = []model.GameEnv{
	{Env: "prod", Description: "Production", Color: "#13c2c2"},
	{Env: "stage", Description: "Staging", Color: "#fa8c16"},
	{Env: "test", Description: "Testing", Color: "#722ed1"},
	{Env: "dev", Description: "Development", Color: "#1677ff"},
}

type gamesBootstrapConfig struct {
	DefaultEnvs []model.GameEnv          `json:"default_envs"`
	Games       []bootstrapGameSeedEntry `json:"games"`
}

type bootstrapGameSeedEntry struct {
	GameID      string          `json:"game_id"`
	Name        string          `json:"name"`
	AliasName   string          `json:"alias_name"`
	DisplayName string          `json:"display_name"`
	Title       string          `json:"title"`
	Icon        string          `json:"icon"`
	Description string          `json:"description"`
	Homepage    string          `json:"homepage"`
	Status      string          `json:"status"`
	Enabled     *bool           `json:"enabled"`
	GameType    string          `json:"game_type"`
	GenreCode   string          `json:"genre_code"`
	Color       string          `json:"color"`
	Env         string          `json:"env"`  // backward-compatible single env
	Envs        []model.GameEnv `json:"envs"` // preferred, rich metadata
}

func seedBootstrapGames(ctx *ServiceContext) error {
	if ctx == nil || ctx.DB == nil || ctx.GameModel == nil {
		return nil
	}

	var existing int64
	if err := ctx.DB.Model(&model.Game{}).Count(&existing).Error; err != nil {
		return fmt.Errorf("count games: %w", err)
	}
	if existing > 0 {
		return nil
	}

	cfg, err := loadGamesBootstrapConfig(ctx.Config)
	if err != nil {
		slog.Default().Error("failed to load games bootstrap config", "error", err)
		cfg = defaultBootstrapGamesConfig()
	}

	defaultEnvs := sanitizeEnvList(cfg.DefaultEnvs)
	if len(cfg.Games) == 0 {
		cfg.Games = []bootstrapGameSeedEntry{defaultBootstrapGame(defaultEnvs)}
	}

	bg := context.Background()
	for idx, entry := range cfg.Games {
		record, err := buildGameFromSeed(entry, defaultEnvs, idx)
		if err != nil {
			slog.Default().Error("skip bootstrap game entry", "index", idx, "error", err)
			continue
		}
		if err := ctx.GameModel.Create(bg, record); err != nil {
			slog.Default().Error("failed to create bootstrap game", "name", record.Name, "error", err)
			continue
		}
		slog.Default().Info("seed bootstrap game", "name", record.Name, "alias", record.AliasName, "gameId", record.GameID)

		// Create env bindings (game_envs table) for database-per-game routing.
		envs, _ := record.GetEnvs()
		for _, env := range envs {
			dbName := router.DefaultGameDBName(record.GameID, env.Env)
			if err := ctx.GameModel.AddEnvBinding(bg, record.GameID, env.Env, dbName, env.Description, env.Color); err != nil {
				slog.Default().Error("failed to create game env binding",
					"gameId", record.GameID, "env", env.Env, "error", err)
			}
		}
	}
	return nil
}

func loadGamesBootstrapConfig(cfg config.Config) (*gamesBootstrapConfig, error) {
	path := resolveGamesConfigPath(cfg)
	if path == "" {
		return nil, errors.New("games config path not found")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Strip UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	var parsed gamesBootstrapConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func resolveGamesConfigPath(c config.Config) string {
	if file := strings.TrimSpace(c.Auth.GamesConfig); file != "" {
		return toAbs(file)
	}
	base := resolveBootstrapBaseDir(c)
	if base == "" {
		base = resolveBootstrapAuthDir(c)
	}
	if base == "" {
		return ""
	}
	candidate := filepath.Join(base, "games.json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func defaultBootstrapGamesConfig() *gamesBootstrapConfig {
	envs := sanitizeEnvList(fallbackDefaultEnvs)
	return &gamesBootstrapConfig{
		DefaultEnvs: envs,
		Games:       []bootstrapGameSeedEntry{defaultBootstrapGame(envs)},
	}
}

func defaultBootstrapGame(envs []model.GameEnv) bootstrapGameSeedEntry {
	return bootstrapGameSeedEntry{
		GameID:      "default",
		AliasName:   "Default Game",
		Description: "Bootstrap placeholder game",
		Status:      "dev",
		Enabled:     boolPtr(true),
		GameType:    "demo",
		GenreCode:   "GEN",
		Color:       defaultGameColor,
		Envs:        append([]model.GameEnv(nil), envs...),
	}
}

func sanitizeEnvList(envs []model.GameEnv) []model.GameEnv {
	if len(envs) == 0 {
		return append([]model.GameEnv(nil), fallbackDefaultEnvs...)
	}
	out := make([]model.GameEnv, 0, len(envs))
	seen := map[string]struct{}{}
	for _, env := range envs {
		name := strings.TrimSpace(env.Env)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, model.GameEnv{
			Env:         name,
			Description: strings.TrimSpace(env.Description),
			Color:       normalizeColor(env.Color, defaultEnvColor(name)),
		})
	}
	if len(out) == 0 {
		return append([]model.GameEnv(nil), fallbackDefaultEnvs...)
	}
	return out
}

func buildGameFromSeed(entry bootstrapGameSeedEntry, defaults []model.GameEnv, index int) (*model.Game, error) {
	gameID := sanitizeGameID(entry.GameID, entry.Name)
	if gameID == "" {
		return nil, errors.New("game_id is required")
	}

	alias := sanitizeAlias(entry.AliasName, entry.DisplayName, entry.Title, entry.Name, humanizeGameID(gameID))
	if alias == "" {
		alias = humanizeGameID(gameID)
	}

	enabled := true
	if entry.Enabled != nil {
		enabled = *entry.Enabled
	}

	status := strings.ToLower(strings.TrimSpace(entry.Status))
	if status == "" {
		status = "dev"
	}

	envRecords := ensureSeedEnvs(entry, defaults)
	if len(envRecords) == 0 {
		envRecords = append([]model.GameEnv(nil), defaults...)
	}

	game := &model.Game{
		GameID:      gameID,
		Name:        gameID,
		AliasName:   alias,
		Description: strings.TrimSpace(entry.Description),
		Icon:        strings.TrimSpace(entry.Icon),
		Homepage:    strings.TrimSpace(entry.Homepage),
		Status:      status,
		Enabled:     enabled,
		GameType:    strings.TrimSpace(entry.GameType),
		GenreCode:   strings.TrimSpace(entry.GenreCode),
		Color:       normalizeColor(entry.Color, pickGameColor(index)),
	}
	if err := game.SetEnvs(envRecords); err != nil {
		return nil, err
	}
	return game, nil
}

func ensureSeedEnvs(entry bootstrapGameSeedEntry, defaults []model.GameEnv) []model.GameEnv {
	if len(entry.Envs) > 0 {
		return sanitizeGameEnvs(entry.Envs, defaults)
	}
	if env := strings.TrimSpace(entry.Env); env != "" {
		return sanitizeGameEnvs([]model.GameEnv{{Env: env}}, defaults)
	}
	return nil
}

func sanitizeGameEnvs(envs []model.GameEnv, defaults []model.GameEnv) []model.GameEnv {
	if len(envs) == 0 {
		return nil
	}
	defaultMap := make(map[string]model.GameEnv, len(defaults))
	for _, env := range defaults {
		defaultMap[strings.ToLower(env.Env)] = env
	}

	out := make([]model.GameEnv, 0, len(envs))
	seen := map[string]struct{}{}
	for _, env := range envs {
		name := strings.TrimSpace(env.Env)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		meta := env
		if meta.Description == "" {
			if def, ok := defaultMap[key]; ok && def.Description != "" {
				meta.Description = def.Description
			}
		}
		meta.Color = normalizeColor(meta.Color, pickEnvColor(name, defaultMap))
		meta.Env = name
		out = append(out, meta)
	}
	return out
}

func sanitizeGameID(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			trimmed = strings.ToLower(trimmed)
			trimmed = strings.ReplaceAll(trimmed, " ", "_")
			return trimmed
		}
	}
	return ""
}

func sanitizeAlias(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func humanizeGameID(id string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ")
	parts := strings.Fields(replacer.Replace(id))
	for i := range parts {
		if len(parts[i]) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func pickGameColor(index int) string {
	if index < len(fallbackGameColorCycle) {
		return fallbackGameColorCycle[index]
	}
	return fallbackGameColorCycle[index%len(fallbackGameColorCycle)]
}

func pickEnvColor(name string, defaults map[string]model.GameEnv) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if def, ok := defaults[key]; ok && def.Color != "" {
		return def.Color
	}
	if color, ok := fallbackEnvColors[key]; ok {
		return color
	}
	return defaultGameColor
}

func defaultEnvColor(name string) string {
	if color, ok := fallbackEnvColors[strings.ToLower(strings.TrimSpace(name))]; ok {
		return color
	}
	return defaultGameColor
}

func normalizeColor(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	if !strings.HasPrefix(trimmed, "#") {
		return trimmed
	}
	return strings.ToLower(trimmed)
}

func boolPtr(v bool) *bool {
	return &v
}
