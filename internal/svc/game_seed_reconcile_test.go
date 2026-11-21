// GAME-013：games.json 存量库对账（gamesSeedReconcile 开关）覆盖。
package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reconcileGamesJSON = `{
  "defaultEnvs": [{"env": "demo", "description": "Demo env"}],
  "games": [
    {"gameId": "demo-game", "aliasName": "Demo Game", "envs": [{"env": "demo"}]},
    {"gameId": "tower_defense", "aliasName": "Tower Defense", "envs": [{"env": "demo"}]}
  ]
}`

func reconcileSvcCtx(t *testing.T, flag bool) *ServiceContext {
	t.Helper()
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "games.json"), []byte(reconcileGamesJSON), 0o644))
	svcCtx.Config.Auth.GamesConfig = filepath.Join(dir, "games.json")
	svcCtx.Config.Auth.GamesSeedReconcile = flag
	return svcCtx
}

// 开关默认关：存量库保持首装仅跑语义，烘焙清单不补种。
func TestSeedBootstrapGames_ReconcileOffKeepsExistingDBUntouched(t *testing.T) {
	svcCtx := reconcileSvcCtx(t, false)
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), &model.Game{
		GameID: "prod_game", Name: "prod_game", AliasName: "Prod Game",
	}))

	require.NoError(t, seedBootstrapGames(svcCtx))

	games, err := svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, "prod_game", games[0].GameID)
}

// 开关打开：缺失的烘焙条目被幂等补种（含 game_envs 绑定），已存在的
// 游戏绝不修改；重复运行无副作用。
func TestSeedBootstrapGames_ReconcileBackfillsMissingEntries(t *testing.T) {
	svcCtx := reconcileSvcCtx(t, true)
	existing := &model.Game{
		GameID: "demo-game", Name: "demo-game", AliasName: "Demo Game (renamed by admin)",
	}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), existing))

	require.NoError(t, seedBootstrapGames(svcCtx))

	// 缺失条目补种。
	games, err := svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, games, 2)

	tower, err := svcCtx.GameModel.FindByGameIDString(context.Background(), "tower_defense")
	require.NoError(t, err)
	assert.Equal(t, "Tower Defense", tower.AliasName)

	// 已存在条目不被覆盖。
	kept, err := svcCtx.GameModel.FindByGameIDString(context.Background(), "demo-game")
	require.NoError(t, err)
	assert.Equal(t, "Demo Game (renamed by admin)", kept.AliasName)

	// 补种条目带环境绑定（database-per-game 路由口径）。
	bindings, err := svcCtx.GameModel.ListEnvBindings(context.Background(), "tower_defense")
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	assert.Equal(t, "demo", bindings[0].Env)
	assert.NotEmpty(t, bindings[0].DatabaseName)

	// 幂等：重复运行不新增、不报错。
	require.NoError(t, seedBootstrapGames(svcCtx))
	games, err = svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, games, 2)
}

// 开关打开 + 空库 = 原全量播种路径（语义不回归）。
func TestSeedBootstrapGames_ReconcileFlagWithEmptyDBSeedsAll(t *testing.T) {
	svcCtx := reconcileSvcCtx(t, true)

	require.NoError(t, seedBootstrapGames(svcCtx))

	games, err := svcCtx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, games, 2)
}
