// 覆盖目标：game service 的 DB 错误分支（Exists/Create/Update/Delete/
// UpdateEnvsAndBindings/FindEnvBinding/ListEnvBindings）、Router 分支
// （deriveGameDBName / ForgetGame / Forget）、enrichedEnvs 的 GetEnvs 与
// 绑定查询失败、EnvUpdate 非法 ID，以及 handler List 的查询参数绑定错误。
package game

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ---- gorm 错误注入基建 ----

var covSchemaCache = &sync.Map{}

func covStmtTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	for _, v := range []interface{}{tx.Statement.Model, tx.Statement.Dest} {
		if v == nil {
			continue
		}
		if s, err := schema.Parse(v, covSchemaCache, schema.NamingStrategy{}); err == nil {
			return s.Table
		}
	}
	return ""
}

type covFailureInjector struct {
	mu      sync.Mutex
	counts  map[string]int
	failAt  map[string]int
	failAll map[string]bool
}

func newCovFailureInjector() *covFailureInjector {
	return &covFailureInjector{
		counts:  map[string]int{},
		failAt:  map[string]int{},
		failAll: map[string]bool{},
	}
}

func (f *covFailureInjector) register(db *gorm.DB) {
	callback := func(op string) func(tx *gorm.DB) {
		return func(tx *gorm.DB) {
			table := covStmtTable(tx)
			if table == "" {
				return
			}
			key := op + ":" + table
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failAll[key] {
				_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				return
			}
			if n, ok := f.failAt[key]; ok {
				f.counts[key]++
				if f.counts[key] >= n {
					_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				}
			}
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register("cov_fail_create", callback("create"))
	_ = db.Callback().Query().Before("gorm:query").Register("cov_fail_query", callback("query"))
	// Scan/Rows 走 Row 处理器（gorm:row），需单独挂接。
	_ = db.Callback().Row().Before("gorm:row").Register("cov_fail_row", callback("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("cov_fail_update", callback("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("cov_fail_delete", callback("delete"))
}

// ---- 独立测试环境 ----

func newGameCovEnv(t *testing.T) (*Service, context.Context, *covFailureInjector, *gorm.DB, *svc.ServiceContext) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{Username: "covadmin", Nickname: "Cov Admin", Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))
	role := &model.Role{Name: "admin"}
	require.NoError(t, roleModel.Create(context.Background(), role))
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))
	require.NoError(t, roleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all", "games:manage", "games:read"}))
	for _, perm := range []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "games:manage", Name: "Games Manage", Resource: "games", Action: "manage", Category: "game"},
		{ID: "games:read", Name: "Games Read", Resource: "games", Action: "read", Category: "game"},
	} {
		require.NoError(t, db.Create(perm).Error)
	}

	inj := newCovFailureInjector()
	inj.register(db)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		GameModel:       model.NewGameModel(db),
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}
	ctx := context.WithValue(context.Background(), "username", "covadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)
	return NewService(svcCtx), ctx, inj, db, svcCtx
}

// seedCovGame 创建一个带 prod 环境的游戏并返回其数字 ID 字符串。
func seedCovGame(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	game := &model.Game{GameID: name, Name: name, Status: "dev", Enabled: true}
	require.NoError(t, model.NewGameModel(db).Create(context.Background(), game))
	require.NoError(t, game.SetEnvs([]model.GameEnv{{Env: "prod", Description: "Production"}}))
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", game.ID).Update("envs", game.Envs).Error)
	return fmt.Sprintf("%d", game.ID)
}

// ---- handler ----

func TestGameCov_HandlerListBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, db := setupGameHandlerTest(t)

	r := gin.New()
	r.Use(addGameAuthMiddleware(db))
	r.GET("/games", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/games?page=abc", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

// ---- service ----

// deriveGameDBName 走 Router 分支：EnvAdd 持久化的绑定使用 Router 命名。
func TestGameCov_EnvAdd_WithRouter(t *testing.T) {
	s, ctx, _, db, svcCtx := newGameCovEnv(t)
	svcCtx.Router = router.New(router.Config{}, db)
	id := seedCovGame(t, db, "covrouter")

	resp, err := s.EnvAdd(ctx, &GameEnvAddRequest{ID: id, Name: "stage2", Type: "预发"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	binding, err := svcCtx.GameModel.FindEnvBinding(context.Background(), "covrouter", "stage2")
	require.NoError(t, err)
	require.NotNil(t, binding)
	assert.Equal(t, router.DefaultGameDBName("covrouter", "stage2"), binding.DatabaseName)
}

// Create：名称唯一性检查查询失败。
func TestGameCov_Create_ExistsCheckError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAt["query:games"] = 1

	resp, err := s.Create(ctx, &GameCreateRequest{Name: "covgame"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Create：games 写入失败。
func TestGameCov_Create_CreateError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAll["create:games"] = true

	resp, err := s.Create(ctx, &GameCreateRequest{Name: "covgame"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Update：重命名时的唯一性检查查询失败。
func TestGameCov_Update_ExistsCheckError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAt["query:games"] = 1

	resp, err := s.Update(ctx, &GameUpdateRequest{ID: id, Name: "covgame2"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Update：games 更新失败。
func TestGameCov_Update_UpdateError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAll["update:games"] = true

	resp, err := s.Update(ctx, &GameUpdateRequest{ID: id, AliasName: "cov-alias"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Delete：游戏存在但 DeleteWithEnvBindings 事务失败。
func TestGameCov_Delete_WithBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAll["delete:games"] = true

	err := s.Delete(ctx, &GameDeleteRequest{ID: id})
	require.Error(t, err)
}

// Delete：携带 Router 时走 ForgetGame 清理缓存连接。
func TestGameCov_Delete_WithRouter(t *testing.T) {
	s, ctx, _, db, svcCtx := newGameCovEnv(t)
	svcCtx.Router = router.New(router.Config{}, db)
	id := seedCovGame(t, db, "covrouterdel")

	require.NoError(t, s.Delete(ctx, &GameDeleteRequest{ID: id}))
}

// Delete：游戏已不存在时兜底 Delete 报错（幂等路径的失败分支）。
func TestGameCov_Delete_MissingGamePlainDeleteError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAll["delete:games"] = true

	err := s.Delete(ctx, &GameDeleteRequest{ID: "999999"})
	require.Error(t, err)
}

// enrichedEnvs：envs JSON 为合法 JSON 但非数组 → GetEnvs 解码失败返回空环境。
// （不能用非法 JSON：缓存层 cachedFetch 会先因 marshal 失败而报错。）
func TestGameCov_EnvsList_BadEnvsJSON(t *testing.T) {
	s, ctx, _, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covbadenvs")
	require.NoError(t, db.Exec("UPDATE games SET envs = ? WHERE id = ?", `{"env":"prod"}`, id).Error)

	resp, err := s.EnvsList(ctx, &GameEnvsListRequest{ID: id})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Envs)
}

// enrichedEnvs：ListEnvBindings 查询失败 → 返回未增强的环境项。
func TestGameCov_EnvsList_BindingsQueryError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covbinderr")
	inj.failAll["query:game_envs"] = true

	resp, err := s.EnvsList(ctx, &GameEnvsListRequest{ID: id})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Envs, 1)
	assert.Empty(t, resp.Envs[0].DatabaseName)
}

// EnvAdd：UpdateEnvsAndBindings 事务失败。
func TestGameCov_EnvAdd_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvadd")
	inj.failAll["update:games"] = true

	resp, err := s.EnvAdd(ctx, &GameEnvAddRequest{ID: id, Name: "stage3"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate：非法数字 ID。
func TestGameCov_EnvUpdate_BadID(t *testing.T) {
	s, ctx, _, _, _ := newGameCovEnv(t)

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: "abc", EnvID: "prod"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate：FindEnvBinding 查询失败。
func TestGameCov_EnvUpdate_FindEnvBindingError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvupd")
	inj.failAll["query:game_envs"] = true

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: id, EnvID: "prod", Type: "生产"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate：UpdateEnvsAndBindings 事务失败。
func TestGameCov_EnvUpdate_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvupd2")
	inj.failAll["update:games"] = true

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: id, EnvID: "prod", Type: "生产"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvDelete：UpdateEnvsAndBindings 事务失败。
func TestGameCov_EnvDelete_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvdel")
	inj.failAll["update:games"] = true

	resp, err := s.EnvDelete(ctx, &GameEnvDeleteRequest{ID: id, EnvID: "prod"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvDelete：携带 Router 时走 Forget 清理该环境缓存连接。
func TestGameCov_EnvDelete_WithRouter(t *testing.T) {
	s, ctx, _, db, svcCtx := newGameCovEnv(t)
	svcCtx.Router = router.New(router.Config{}, db)
	id := seedCovGame(t, db, "covrouterenv")

	resp, err := s.EnvDelete(ctx, &GameEnvDeleteRequest{ID: id, EnvID: "prod"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Envs)
}
