package game

// 覆盖率巡检第十五轮（wt-api）：helpers.go 残余 3 块收口——
// gameEnvScopes 的 LoadCurrentAdmin 失败翼（上下文无身份）与
// GetAdminEnvScopes 存储错误翼（授权表缺表 → errorx 内部错误，不裸传
// SQL 错误）、authorizeGameEnv 对 gameEnvScopes 错误的透传翼。
// 复用 service_envscope_test.go 夹具（#47 授权防线域，本会话旧域）。

import (
	"context"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGameEnvScopes_ErrorWings(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	service := NewService(svcCtx)

	// LoadCurrentAdmin 失败翼：上下文无身份信息
	_, _, err := service.gameEnvScopes(context.Background(), 1)
	require.Error(t, err)

	// GetAdminEnvScopes 存储错误翼：授权表缺表 → 包装为内部错误。
	// setupTestDB 是 cache=shared 进程级共享库（DropTable 会毒化他用例），
	// 缺表注入须用独立库。
	iso, err := gorm.Open(gsqlite.Open("file:game_envscope_wings?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(iso))
	seedTestPermissions(t, iso)
	svcCtx2 := setupTestServiceContext(t, iso)
	ctx, _ := createEnvScopedManager(t, iso, "scope_store_broken")
	require.NoError(t, iso.Migrator().DropTable("admin_game_env_scopes"))
	service2 := NewService(svcCtx2)

	_, _, err = service2.gameEnvScopes(ctx, 1)
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	assert.True(t, ok, "存储故障应包装为 CodeError，got %T: %v", err, err)

	// authorizeGameEnv 透传翼：底层 gameEnvScopes 错误原样上抛
	err = service2.authorizeGameEnv(ctx, 1, "prod")
	require.Error(t, err)

	// EnvsList 透传翼：权限与游戏寻址都过后撞上 gameEnvScopes 存储故障
	game := createScopeTestGame(t, iso, "envlist_store_broken", model.GameEnv{Env: "prod"})
	_, err = service2.EnvsList(ctx, &GameEnvsListRequest{ID: strconv.FormatUint(uint64(game.ID), 10)})
	require.Error(t, err)
}
