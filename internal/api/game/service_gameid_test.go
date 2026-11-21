// GAME-013：games REST 显式 gameId 支持（Create/Update 的连字符保留、
// 唯一性、改名守卫）覆盖。
package game

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGameServiceTestCtx(t *testing.T) (context.Context, *Service) {
	t.Helper()
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	ctx, _ := createTestAdminWithContext(t, db, "gameid-admin", "password123", "admin")
	return ctx, NewService(svcCtx)
}

// 显式 gameId 连字符原样保留，与 SDK 侧注册口径（demo-game）一致；
// 业务串寻址可解析到该游戏。
func Test_Create_ExplicitGameIDWithHyphen(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:        "DemoGame",
		AliasName:   "Demo Game",
		GameID:      "demo-game",
		Description: "explicit hyphenated game id",
	})

	require.NoError(t, err)
	assert.Equal(t, "demo-game", resp.Game.GameID)
	assert.Equal(t, "DemoGame", resp.Game.Name)

	// 业务串寻址（X-Game-ID 同口径）可解析。
	found, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "demo-game")
	require.NoError(t, err)
	assert.Equal(t, resp.Game.ID, found.ID)
}

// 未显式提交 gameId 时保持既有语义：Name 派生，连字符归一为下划线。
func Test_Create_GameIDDerivedFromNameNormalizesHyphen(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	resp, err := service.Create(ctx, &GameCreateRequest{Name: "demo-game", AliasName: "Derived"})

	require.NoError(t, err)
	assert.Equal(t, "demo_game", resp.Game.GameID)
}

// 显式 gameId 非法字符集 / 长度超界被拒绝。
func Test_Create_ExplicitGameID_InvalidCharacters(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	for i, bad := range []string{"bad id!", "demo/game", "demo@game", "demo game!!"} {
		resp, err := service.Create(ctx, &GameCreateRequest{
			Name:      "game_for_bad_" + strconv.Itoa(i),
			AliasName: "bad",
			GameID:    bad,
		})
		assert.Error(t, err, "input %q", bad)
		assert.Nil(t, resp)
	}
}

// 显式 gameId 重复（大小写不敏感归一后冲突）被 409 拒绝。
func Test_Create_ExplicitGameID_Duplicate(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "first", AliasName: "First", GameID: "demo-game"})
	require.NoError(t, err)

	// 同名不同写法（大写归一 + 连字符）冲突。
	resp, err := service.Create(ctx, &GameCreateRequest{Name: "second", AliasName: "Second", GameID: "Demo-Game"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// 未绑定环境的游戏可改 game_id；改名后旧标识不可寻址、新标识可寻址。
func Test_Update_RenameGameID_Success(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "renamable", AliasName: "Renamable"})
	require.NoError(t, err)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(created.Game.ID)),
		GameID:    "renamable-game",
		AliasName: "Renamable",
	})
	require.NoError(t, err)
	assert.Equal(t, "renamable-game", resp.Game.GameID)

	_, err = service.svcCtx.GameModel.FindByGameIDString(ctx, "renamable")
	assert.ErrorIs(t, err, model.ErrGameNotFound)
	found, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "renamable-game")
	require.NoError(t, err)
	assert.Equal(t, created.Game.ID, found.ID)
}

// 游戏已绑定环境时拒绝改 game_id（防路由数据悬空）。
func Test_Update_RenameGameID_BlockedByEnvBinding(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "bound-game", AliasName: "Bound"})
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.GameModel.AddEnvBinding(
		ctx, created.Game.GameID, "demo", "game_bound_game_demo", "Demo env", "#13c2c2",
	))

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(created.Game.ID)),
		GameID:    "bound-game-new",
		AliasName: "Bound",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// 游戏 envs 元数据非空（尚未 backfill 绑定的过渡态）同样拒绝改 game_id。
func Test_Update_RenameGameID_BlockedByEnvsMetadata(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	created, err := service.Create(ctx, &GameCreateRequest{Name: "envs-meta-game", AliasName: "EnvsMeta"})
	require.NoError(t, err)
	_ = created

	game, err := service.svcCtx.GameModel.FindByGameIDString(ctx, "envs_meta_game")
	require.NoError(t, err)
	envsJSON, err := json.Marshal([]model.GameEnv{{Env: "demo", Description: "Demo env"}})
	require.NoError(t, err)
	require.NoError(t, service.svcCtx.GameModel.Update(ctx, game.ID, map[string]interface{}{"envs": envsJSON}))

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(game.ID)),
		GameID:    "envs-meta-game-new",
		AliasName: "EnvsMeta",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// 改 game_id 撞已有标识被 409 拒绝。
func Test_Update_RenameGameID_Duplicate(t *testing.T) {
	ctx, service := newGameServiceTestCtx(t)

	_, err := service.Create(ctx, &GameCreateRequest{Name: "taken-id", AliasName: "Taken", GameID: "taken-game"})
	require.NoError(t, err)
	target, err := service.Create(ctx, &GameCreateRequest{Name: "rename-target", AliasName: "RenameTarget"})
	require.NoError(t, err)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.Itoa(int(target.Game.ID)),
		GameID:    "taken-game",
		AliasName: "RenameTarget",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}
