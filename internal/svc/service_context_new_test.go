package svc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSvcConfig(t *testing.T, multiGame bool) config.Config {
	t.Helper()
	dir := t.TempDir()
	return config.Config{
		Server: config.ServerConfig{Mode: "dev"},
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: filepath.Join(dir, "meta.db"),
			MultiGame:  multiGame,
		},
		BootstrapData: config.BootstrapDataConfig{BaseDir: filepath.Join(dir, "bootstrap")},
	}
}

func TestNewServiceContext_SingleDB(t *testing.T) {
	ctx := NewServiceContext(newSvcConfig(t, false))

	require.NotNil(t, ctx)
	require.NotNil(t, ctx.DB)
	assert.Nil(t, ctx.Router)
	require.NotNil(t, ctx.AdminModel)
	require.NotNil(t, ctx.GameModel)
	require.NotNil(t, ctx.CacheHelper)
	require.NotNil(t, ctx.MetricsStore)
	require.NotNil(t, ctx.SystemInfoCache)
	require.NotNil(t, ctx.Authority)
	require.NotNil(t, ctx.AdminManager)

	// 默认引导数据已生成
	games, err := ctx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, games)

	// 引导管理员可登录验证
	require.NotNil(t, ctx.AdminManager.ListAdmins())
}

func TestNewServiceContext_MultiGame(t *testing.T) {
	ctx := NewServiceContext(newSvcConfig(t, true))

	require.NotNil(t, ctx)
	require.NotNil(t, ctx.Router)

	// game_envs 引导绑定存在，可通过 router 打开游戏库
	bindings, err := ctx.GameModel.ListAllEnvBindings(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, bindings)

	gameDB, err := ctx.Router.GameDB(context.Background(), bindings[0].GameID, bindings[0].Env)
	require.NoError(t, err)
	require.NotNil(t, gameDB)
}

func TestNewServiceContext_PanicsOnBadDatabase(t *testing.T) {
	cfg := newSvcConfig(t, false)
	cfg.Database.Driver = "unsupported-driver"
	assert.Panics(t, func() { NewServiceContext(cfg) })
}

func TestServiceContext_ScopeResolutionThroughRouter(t *testing.T) {
	ctx := NewServiceContext(newSvcConfig(t, true))

	// 通过 background 注册 scope 上下文解析游戏库
	out := ctx.scopeContextForBackgroundRegistration("demo", "prod")
	assert.Equal(t, "demo", GameScopeFromContext(out).GameID)
}
