package svc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestGameDBMiddleware_PersistedScopeRoutesByBindingDatabaseName 是 #2 的验收：
// 无头请求走 persisted last-scope 分支时，binding 的自定义 database_name 必须
// 被复用于物理路由（而非重算 game_<gameID>_<env> 前缀，或二次查库后仍取错名）。
// 这正是「persisted-scope 校验一次即得物理库名并复用」分支（bindingDBName 非空
// 跳过二次 validateGameScope）的可观测行为面。
func TestGameDBMiddleware_PersistedScopeRoutesByBindingDatabaseName(t *testing.T) {
	ctx := context.Background()
	svcCtx := setupTestServiceContext(t)

	require.NoError(t, svcCtx.GameModel.Create(ctx, &model.Game{Name: "Demo", GameID: "demo", AliasName: "demo"}))
	// 自定义名，刻意不同于默认 game_demo_prod 与 seedGameWithEnv 的 db_demo_prod。
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(ctx, "demo", "prod", "persisted_binding_db", "", ""))

	// 管理员 + super_admin 角色（authorizeScope 旁路），并持久化 last scope。
	admin := &model.Admin{Username: "reuse-admin", Status: 1}
	require.NoError(t, svcCtx.AdminModel.Create(ctx, admin, "password"))
	role := &model.Role{Name: "super_admin"}
	require.NoError(t, svcCtx.RoleModel.Create(ctx, role))
	require.NoError(t, svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))
	require.NoError(t, svcCtx.DB.Model(admin).Updates(map[string]any{"last_game_id": "demo", "last_env": "prod"}).Error)

	// Router 记录实际打开的物理库名。
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
	c.Set("adminID", admin.ID)
	c.Request = httptest.NewRequest(http.MethodGet, "/scoped", nil) // 无 X-Game-ID/X-Env 头

	GameDBMiddleware(svcCtx)(c)
	assert.False(t, c.IsAborted())
	require.Len(t, opened, 1, "中间件应恰好打开一个物理库（bindingDBName 复用路径）")
	assert.Equal(t, "persisted_binding_db", opened[0], "必须路由到 binding 自定义名，而非重算前缀")
}
