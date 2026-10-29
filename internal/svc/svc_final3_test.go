package svc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitObjectStore_SignedURLTTLVariants(t *testing.T) {
	ctx := context.Background()
	base := filepath.Join(t.TempDir(), "uploads")

	store, err := initObjectStore(ctx, config.StorageConfig{
		Driver: "file", BaseDir: base, SignedURLTTL: "30s",
	})
	require.NoError(t, err)
	require.NotNil(t, store)

	// An unparsable TTL is ignored and the driver default applies.
	store, err = initObjectStore(ctx, config.StorageConfig{
		Driver: "file", BaseDir: base, SignedURLTTL: "bogus",
	})
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func TestSeedBootstrapManagers_NilAndEmptyEntries(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	managerDir := t.TempDir()
	manager := NewAdminManager(managerDir)

	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "roles.json"), []byte(
		`[null,{"code":"","name":"Empty"},{"code":"ops","name":"Ops","permissions":[]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managerDir, "admins.json"), []byte(
		`[{"username":"mixed","password":"secret123","roles":["","ops"],"status":1}]`), 0o644))
	require.NoError(t, manager.Initialize())
	svcCtx.AdminManager = manager

	require.NoError(t, seedBootstrapPermissions(svcCtx))
	require.NoError(t, seedBootstrapRoles(svcCtx))
	require.NoError(t, seedBootstrapAdmins(svcCtx))
}

func TestResolveFirstAuthorizedGame_OrderingAndMissingGame(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	admin := &model.Admin{Username: "root2", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "password"))
	role := &model.Role{Name: "super_admin"}
	require.NoError(t, svcCtx.RoleModel.Create(ctx, role))
	require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))

	// Admin with no bindings at all -> empty result.
	gameID, env := resolveFirstAuthorizedGame(ctx, svcCtx, admin.ID)
	assert.Empty(t, gameID)
	assert.Empty(t, env)

	// Same game, two environments: the env comparison arm of the sorter runs
	// and "dev" sorts before "prod".
	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Beta", GameID: "beta", AliasName: "beta"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "beta", "prod", "db_beta_prod", "", ""))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "beta", "dev", "db_beta_dev", "", ""))
	gameID, env = resolveFirstAuthorizedGame(ctx, svcCtx, admin.ID)
	assert.Equal(t, "beta", gameID)
	assert.Equal(t, "dev", env)

	// A binding whose game row vanished yields an empty fallback.
	player := &model.Admin{Username: "plain", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, player, "password"))
	require.NoError(t, svcCtx.AdminModel.SetGameEnvScope(ctx, player.ID, 424242, "prod"))
	gameID, env = resolveFirstAuthorizedGame(ctx, svcCtx, player.ID)
	assert.Empty(t, gameID)
	assert.Empty(t, env)
}

func TestGameDBMiddleware_InjectsGameDatabase(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Demo", GameID: "demo", AliasName: "demo"}))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "demo", "prod", "db_demo_prod", "", ""))

	metaDSN := filepath.Join(t.TempDir(), "meta.db")
	svcCtx.Router = newGameRouter(config.Config{
		Database: config.DatabaseConfig{Driver: "sqlite", DataSource: metaDSN},
	}, svcCtx.DB, nil)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/scoped", nil)
	c.Request.Header.Set(GameDBHeader, "demo")
	c.Request.Header.Set(EnvHeader, "prod")

	GameDBMiddleware(svcCtx)(c)
	assert.False(t, c.IsAborted())
	scope := GameScopeFromContext(c.Request.Context())
	assert.Equal(t, "demo", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}

// TestGameDBMiddleware_RoutesByBindingDatabaseName 是 ARCH-006 的中间件级
// 验收：binding 的自定义 database_name 才是物理路由真值。中间件把授权后的
// binding 名传给 GameDBByPhysicalName，打开的物理库必须正是 binding 名，
// 而非重算的 game_<gameID>_<env>（旧实现会打开错误的库）。
func TestGameDBMiddleware_RoutesByBindingDatabaseName(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Demo", GameID: "demo", AliasName: "demo"}))
	// 自定义名，刻意不同于默认 game_demo_prod。
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "demo", "prod", "binding_truth_db", "", ""))

	// t.TempDir() 每次调用返回同一路径；EnsureDatabase 的返回值会覆盖
	// DSNForDatabase 的 DSN，两侧必须用同一绝对路径，否则 sqlite 会在
	// 测试进程 CWD 落盘泄漏文件。
	gameDir := t.TempDir()
	var (
		mu     sync.Mutex
		opened []string
	)
	svcCtx.Router = router.New(router.Config{
		Driver: "sqlite",
		DSNForDatabase: func(metaDSN, dbName string) string {
			return filepath.Join(gameDir, dbName+".db")
		},
		EnsureDatabase: func(_ context.Context, _, _, dbName string) (string, error) {
			mu.Lock()
			opened = append(opened, dbName)
			mu.Unlock()
			return filepath.Join(gameDir, dbName+".db"), nil
		},
		Open: func(_ context.Context, _, dsn string) (*gorm.DB, error) {
			return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
		},
	}, svcCtx.DB)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/scoped", nil)
	c.Request.Header.Set(GameDBHeader, "demo")
	c.Request.Header.Set(EnvHeader, "prod")

	GameDBMiddleware(svcCtx)(c)
	assert.False(t, c.IsAborted())
	require.Len(t, opened, 1, "中间件应恰好打开一个物理库")
	assert.Equal(t, "binding_truth_db", opened[0], "必须路由到 binding 自定义名，而非重算 game_demo_prod")
}
