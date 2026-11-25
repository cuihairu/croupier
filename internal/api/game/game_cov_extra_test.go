// Coverage targets: the game service's DB error branches (Exists/Create/Update/Delete/
// UpdateEnvsAndBindings/FindEnvBinding/ListEnvBindings), the Router branches
// (deriveGameDBName / ForgetGame / Forget), GetEnvs of enrichedEnvs and binding
// query failures, EnvUpdate with an invalid ID, and the handler List query
// parameter binding error.
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

// ---- gorm error injection infrastructure ----

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
	// Scan/Rows go through the Row processor (gorm:row) and need a separate hook.
	_ = db.Callback().Row().Before("gorm:row").Register("cov_fail_row", callback("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("cov_fail_update", callback("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("cov_fail_delete", callback("delete"))
}

// ---- isolated test environment ----

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

// seedCovGame creates a game with a prod environment and returns its numeric ID as a string.
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

// deriveGameDBName takes the Router branch: bindings persisted by EnvAdd use Router naming.
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

// Create: query for the name uniqueness check fails.
func TestGameCov_Create_ExistsCheckError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAt["query:games"] = 1

	resp, err := s.Create(ctx, &GameCreateRequest{Name: "covgame"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Create: writing the games row fails.
func TestGameCov_Create_CreateError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAll["create:games"] = true

	resp, err := s.Create(ctx, &GameCreateRequest{Name: "covgame"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Update: query for the rename uniqueness check fails.
func TestGameCov_Update_ExistsCheckError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAt["query:games"] = 1

	resp, err := s.Update(ctx, &GameUpdateRequest{ID: id, Name: "covgame2"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Update: updating the games row fails.
func TestGameCov_Update_UpdateError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAll["update:games"] = true

	resp, err := s.Update(ctx, &GameUpdateRequest{ID: id, AliasName: "cov-alias"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// Delete: the game exists but the DeleteWithEnvBindings transaction fails.
func TestGameCov_Delete_WithBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covgame")
	inj.failAll["delete:games"] = true

	err := s.Delete(ctx, &GameDeleteRequest{ID: id})
	require.Error(t, err)
}

// Delete: with a Router present, ForgetGame clears cached connections.
func TestGameCov_Delete_WithRouter(t *testing.T) {
	s, ctx, _, db, svcCtx := newGameCovEnv(t)
	svcCtx.Router = router.New(router.Config{}, db)
	id := seedCovGame(t, db, "covrouterdel")

	// Environments not emptied -> deletion refused (deletion gate)
	require.Error(t, s.Delete(ctx, &GameDeleteRequest{ID: id}))

	// Clear the Envs metadata -> deletion is let through, covering the Router forget path
	require.NoError(t, db.Model(&model.Game{}).Where("id = ?", id).Update("envs", "[]").Error)
	require.NoError(t, s.Delete(ctx, &GameDeleteRequest{ID: id}))
}

// Delete: fallback Delete errors when the game is already gone (failure branch of the idempotent path).
func TestGameCov_Delete_MissingGamePlainDeleteError(t *testing.T) {
	s, ctx, inj, _, _ := newGameCovEnv(t)
	inj.failAll["delete:games"] = true

	err := s.Delete(ctx, &GameDeleteRequest{ID: "999999"})
	require.Error(t, err)
}

// enrichedEnvs: envs JSON is valid JSON but not an array -> GetEnvs decoding fails and returns no envs.
// (Invalid JSON will not do: the cache layer's cachedFetch errors out first on the marshal failure.)
func TestGameCov_EnvsList_BadEnvsJSON(t *testing.T) {
	s, ctx, _, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covbadenvs")
	require.NoError(t, db.Exec("UPDATE games SET envs = ? WHERE id = ?", `{"env":"prod"}`, id).Error)

	resp, err := s.EnvsList(ctx, &GameEnvsListRequest{ID: id})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Envs)
}

// enrichedEnvs: ListEnvBindings query fails -> unenriched env items are returned.
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

// EnvAdd: UpdateEnvsAndBindings transaction fails.
func TestGameCov_EnvAdd_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvadd")
	inj.failAll["update:games"] = true

	resp, err := s.EnvAdd(ctx, &GameEnvAddRequest{ID: id, Name: "stage3"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate: invalid numeric ID.
func TestGameCov_EnvUpdate_BadID(t *testing.T) {
	s, ctx, _, _, _ := newGameCovEnv(t)

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: "abc", EnvID: "prod"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate: FindEnvBinding query fails.
func TestGameCov_EnvUpdate_FindEnvBindingError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvupd")
	inj.failAll["query:game_envs"] = true

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: id, EnvID: "prod", Type: "生产"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvUpdate: UpdateEnvsAndBindings transaction fails.
func TestGameCov_EnvUpdate_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvupd2")
	inj.failAll["update:games"] = true

	resp, err := s.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: id, EnvID: "prod", Type: "生产"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvDelete: UpdateEnvsAndBindings transaction fails.
func TestGameCov_EnvDelete_UpdateEnvsAndBindingsError(t *testing.T) {
	s, ctx, inj, db, _ := newGameCovEnv(t)
	id := seedCovGame(t, db, "covenvdel")
	inj.failAll["update:games"] = true

	resp, err := s.EnvDelete(ctx, &GameEnvDeleteRequest{ID: id, EnvID: "prod"})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// EnvDelete: with a Router present, Forget clears that environment's cached connection.
func TestGameCov_EnvDelete_WithRouter(t *testing.T) {
	s, ctx, _, db, svcCtx := newGameCovEnv(t)
	svcCtx.Router = router.New(router.Config{}, db)
	id := seedCovGame(t, db, "covrouterenv")

	resp, err := s.EnvDelete(ctx, &GameEnvDeleteRequest{ID: id, EnvID: "prod"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Envs)
}
