package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newClosedServiceContext builds a service context whose database pool has
// been closed so every loader hits its error branch.
func newClosedServiceContext(t *testing.T) *ServiceContext {
	t.Helper()
	svcCtx := setupTestServiceContext(t)
	sqlDB, err := svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return svcCtx
}

func TestCacheLayer_LoaderErrors(t *testing.T) {
	ctx := context.Background()
	svcCtx := newClosedServiceContext(t)

	_, err := svcCtx.GetAdminCached(ctx, 42)
	assert.Error(t, err)

	admin, err := svcCtx.GetAdminByUsernameCached(ctx, "ghost")
	assert.Error(t, err)
	assert.Nil(t, admin)

	_, err = svcCtx.GetAdminRolesCached(ctx, 1)
	assert.Error(t, err)

	role, err := svcCtx.GetRoleCached(ctx, 7)
	assert.Error(t, err)
	assert.Nil(t, role)

	_, err = svcCtx.GetRolePermissionIDsCached(ctx, 7)
	assert.Error(t, err)

	perm, err := svcCtx.GetPermissionCached(ctx, "player:view")
	assert.Error(t, err)
	assert.Nil(t, perm)

	game, err := svcCtx.GetGameCached(ctx, 9)
	assert.Error(t, err)
	assert.Nil(t, game)

	games, err := svcCtx.ListAllGamesCached(ctx)
	assert.Error(t, err)
	assert.Nil(t, games)
}

func TestCacheLayer_EmptyInputsReturnNil(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	admin, err := svcCtx.GetAdminByUsernameCached(ctx, "   ")
	assert.NoError(t, err)
	assert.Nil(t, admin)

	perm, err := svcCtx.GetPermissionCached(ctx, "")
	assert.NoError(t, err)
	assert.Nil(t, perm)

	// Invalidate helpers must be safe with empty identifiers.
	svcCtx.InvalidateAdminCache(ctx, 0, "")
	svcCtx.InvalidatePermissionCache(ctx, "")
}

func TestOpsStateStore_CorruptFileFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ops_state.json"), []byte("{not json"), 0o644))

	store := NewOpsStateStore(dir)
	state := store.Snapshot()
	assert.Equal(t, "redis", state.MQ.Type)
}

func TestOpsStateStore_SaveFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	// The store path lives underneath a regular file: MkdirAll fails.
	store := &OpsStateStore{path: filepath.Join(blocker, "ops_state.json"), state: defaultOpsState()}
	_, err := store.Update(func(state *OpsState) { state.MQ.Type = "kafka" })
	assert.Error(t, err)

	err = store.load()
	assert.Error(t, err)
}

func TestSeedBootstrapGames_CountError(t *testing.T) {
	svcCtx := newClosedServiceContext(t)
	assert.Error(t, seedBootstrapGames(svcCtx))
}

func TestSeedBootstrapGames_WithSeedConfig(t *testing.T) {
	baseDir := t.TempDir()
	gamesJSON := `{"defaultEnvs":[{"env":"dev","description":"Development"}],"games":[{"gameId":"Demo Game","aliasName":"Demo","status":"prod","color":"#123456","envs":[{"env":"prod"},{"env":""},{"env":"prod"}]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "games.json"), []byte(gamesJSON), 0o644))

	svcCtx := setupTestServiceContext(t)
	svcCtx.Config.Auth.GamesConfig = filepath.Join(baseDir, "games.json")
	require.NoError(t, seedBootstrapGames(svcCtx))

	games, err := svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, "demo_game", games[0].GameID)
}

func TestLoadGamesBootstrapConfig_BOMAndInvalid(t *testing.T) {
	dir := t.TempDir()

	bomPath := filepath.Join(dir, "bom.json")
	payload := []byte{0xEF, 0xBB, 0xBF}
	payload = append(payload, `{"games":[{"gameId":"g1"}]}`...)
	require.NoError(t, os.WriteFile(bomPath, payload, 0o644))

	cfg := config.Config{Auth: config.AuthConfig{GamesConfig: bomPath}}
	parsed, err := loadGamesBootstrapConfig(cfg)
	require.NoError(t, err)
	require.Len(t, parsed.Games, 1)

	badPath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(badPath, []byte("{nope"), 0o644))
	cfg.Auth.GamesConfig = badPath
	_, err = loadGamesBootstrapConfig(cfg)
	assert.Error(t, err)

	missing := config.Config{Auth: config.AuthConfig{GamesConfig: filepath.Join(dir, "missing.json")}}
	_, err = loadGamesBootstrapConfig(missing)
	assert.Error(t, err)

	empty := config.Config{}
	_, err = loadGamesBootstrapConfig(empty)
	assert.Error(t, err)
}

func TestResolveGamesConfigPath_BaseDirCandidate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "games.json"), []byte("{}"), 0o644))

	path := resolveGamesConfigPath(config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}})
	assert.Equal(t, filepath.Join(dir, "games.json"), path)

	// No candidate anywhere -> empty path (not an error).
	assert.Empty(t, resolveGamesConfigPath(config.Config{}))
}

func TestSanitizeEnvList_AllBlankFallsBackToDefaults(t *testing.T) {
	out := sanitizeEnvList([]model.GameEnv{{Env: ""}, {Env: "   "}})
	assert.NotEmpty(t, out)
}

func TestBuildGameFromSeed_ErrorsAndFallbacks(t *testing.T) {
	defaults := sanitizeEnvList(nil)

	_, err := buildGameFromSeed(bootstrapGameSeedEntry{}, defaults, 0)
	assert.Error(t, err)

	// Index beyond the color cycle wraps around.
	game, err := buildGameFromSeed(bootstrapGameSeedEntry{
		GameID: "my_game", Name: "", AliasName: "", DisplayName: "", Title: "",
		Color: "#ABCDEF",
	}, defaults, len(fallbackGameColorCycle)+3)
	require.NoError(t, err)
	assert.Equal(t, "#abcdef", game.Color)
	assert.Equal(t, "My Game", game.AliasName)

	humanized := humanizeGameID("multi-player_zone")
	assert.Equal(t, "Multi Player Zone", humanized)
	assert.Empty(t, sanitizeGameID("", "  "))
}

func TestSeedBootstrapTermDictionary_Variants(t *testing.T) {
	ctx := context.Background()

	assert.NoError(t, seedBootstrapTermDictionary(nil))

	// Model without a database fails on the first upsert.
	closedCtx := newClosedServiceContext(t)
	assert.Error(t, seedBootstrapTermDictionary(closedCtx))

	// Invalid seed payloads are rejected before touching the database.
	baseDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "term_dictionary.json"), []byte(
		`{"items":[{"domain":"","key":"player"}]}`), 0o644))
	svcCtx := setupTestServiceContext(t)
	svcCtx.Config.BootstrapData.BaseDir = baseDir
	err := seedBootstrapTermDictionary(svcCtx)
	assert.Error(t, err)

	// Missing alias inside the aliases list is rejected as well.
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "term_dictionary.json"), []byte(
		`{"items":[{"domain":"resource","key":"player","aliases":[""]}]}`), 0o644))
	err = seedBootstrapTermDictionary(svcCtx)
	assert.Error(t, err)

	// A valid custom seed file upserts every entry.
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "term_dictionary.json"), []byte(
		`{"items":[{"domain":"resource","key":"vip","aliases":["VIP"],"displayZh":"会员","displayEn":"VIP","order":5}]}`), 0o644))
	require.NoError(t, seedBootstrapTermDictionary(svcCtx))
	items, err := svcCtx.TermDictModel.List(ctx, "resource")
	require.NoError(t, err)
	found := false
	for _, item := range items {
		if item.Alias == "vip" && item.DisplayZh == "会员" {
			found = true
		}
	}
	assert.True(t, found)

	// Corrupt JSON falls back to the built-in defaults.
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "term_dictionary.json"), []byte("{bad"), 0o644))
	cfg := loadTermDictionaryConfig(svcCtx)
	assert.NotEmpty(t, cfg.Items)
}
