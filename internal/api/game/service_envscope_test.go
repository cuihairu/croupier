package game

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// createEnvScopedManager 建一个非 admin 角色的运营者：持有 games:manage 功能
// 权限点（能过 RequireAnyPermission），但游戏/环境可见性由 admin_game_env_scopes
// 授权表决定（gameEnvScopes 防线的数据源）。
func createEnvScopedManager(t *testing.T, db *gorm.DB, username string) (context.Context, uint) {
	t.Helper()

	adminModel := model.NewAdminModel(db)
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, adminModel.Create(context.Background(), admin, "password123"))

	role := &model.Role{Name: "env-manager-" + username, Description: "Env scoped manager"}
	require.NoError(t, db.Where("name = ?", role.Name).FirstOrCreate(role).Error)
	rolePerm := &model.RolePermission{RoleID: role.ID, PermissionID: "games:manage"}
	require.NoError(t, db.Where("role_id = ? AND permission_id = ?", role.ID, "games:manage").
		FirstOrCreate(rolePerm).Error)
	require.NoError(t, adminModel.AssignRole(context.Background(), admin.ID, role.ID))

	ctx := context.WithValue(context.Background(), "username", username)
	return context.WithValue(ctx, "adminID", admin.ID), admin.ID
}

// seedEnvScope 授予 (adminID, gameID, env) 的环境授权。
func seedEnvScope(t *testing.T, db *gorm.DB, adminID, gameID uint, env string) {
	t.Helper()
	require.NoError(t, db.Create(&model.AdminGameEnvScope{AdminID: adminID, GameID: gameID, Env: env}).Error)
}

func createScopeTestGame(t *testing.T, db *gorm.DB, name string, envs ...model.GameEnv) *model.Game {
	t.Helper()
	gameModel := model.NewGameModel(db)
	game := &model.Game{Name: name, AliasName: name, Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game))
	if len(envs) > 0 {
		require.NoError(t, game.SetEnvs(envs))
		require.NoError(t, gameModel.Update(context.Background(), game.ID,
			map[string]interface{}{"envs": game.Envs}))
	}
	return game
}

func TestService_EnvAdd_ForbiddenWithoutGameScope(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_add_denied")

	// 持 games:manage 但无任何游戏授权 → 写路径拒绝（功能权限点不再是唯一防线）
	ctx, _ := createEnvScopedManager(t, db, "manager_no_scope")
	service := NewService(svcCtx)

	_, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "staging",
	})
	require.Error(t, err)
	forbidden, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
}

func TestService_EnvAdd_SuccessWithGameScope(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_add_allowed",
		model.GameEnv{Env: "prod", Description: "Production"})

	// 游戏维度有授权（任一环境）即可新增环境（新增环境尚不存在、不在授权表内）
	ctx, adminID := createEnvScopedManager(t, db, "manager_add")
	seedEnvScope(t, db, adminID, game.ID, "prod")
	service := NewService(svcCtx)

	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "staging",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Envs, 2)
}

func TestService_EnvUpdate_RequiresBothEnvsInScope(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_update",
		model.GameEnv{Env: "prod", Description: "Production"},
		model.GameEnv{Env: "dev", Description: "Development"})
	gameID := strconv.FormatUint(uint64(game.ID), 10)

	ctx, adminID := createEnvScopedManager(t, db, "manager_update")
	seedEnvScope(t, db, adminID, game.ID, "prod")
	service := NewService(svcCtx)

	// 只改描述（名字不变）：目标环境 prod 在授权内 → 通过
	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID: gameID, EnvID: "prod", Name: "prod", Type: "Updated description",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Envs, 2)

	// 目标环境 dev 不在授权内 → 拒绝
	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID: gameID, EnvID: "dev", Type: "Sneaky edit",
	})
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	ce, _ := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusForbidden, ce.Code)

	// 改名 prod → beta：beta 不在授权内，改名即扩权 → 拒绝
	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID: gameID, EnvID: "prod", Name: "beta",
	})
	require.Error(t, err)
	_, ok = err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
}

func TestService_EnvDelete_RequiresEnvInScope(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_delete",
		model.GameEnv{Env: "prod", Description: "Production"},
		model.GameEnv{Env: "dev", Description: "Development"})
	gameID := strconv.FormatUint(uint64(game.ID), 10)

	ctx, adminID := createEnvScopedManager(t, db, "manager_delete")
	seedEnvScope(t, db, adminID, game.ID, "prod")
	service := NewService(svcCtx)

	_, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{ID: gameID, EnvID: "dev"})
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	ce, _ := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusForbidden, ce.Code)

	resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{ID: gameID, EnvID: "prod"})
	require.NoError(t, err)
	assert.Len(t, resp.Envs, 1)
}

func TestService_EnvsList_FiltersByScope(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_list",
		model.GameEnv{Env: "prod", Description: "Production"},
		model.GameEnv{Env: "dev", Description: "Development"})
	gameID := strconv.FormatUint(uint64(game.ID), 10)

	ctx, adminID := createEnvScopedManager(t, db, "manager_list")
	seedEnvScope(t, db, adminID, game.ID, "prod")
	service := NewService(svcCtx)

	// 只授权 prod：列表收敛到 prod，dev 不可见
	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: gameID})
	require.NoError(t, err)
	require.Len(t, resp.Envs, 1)
	assert.Equal(t, "prod", resp.Envs[0].Env)
}

func TestService_EnvsList_BusinessKeyAddressing(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	game := createScopeTestGame(t, db, "scope_bizkey",
		model.GameEnv{Env: "prod", Description: "Production"})

	// 前端授权视图（/profile/games）无数字主键：业务 game_id 寻址等价可用
	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")
	service := NewService(svcCtx)
	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: game.GameID})
	require.NoError(t, err)
	require.Len(t, resp.Envs, 1)
	assert.Equal(t, "prod", resp.Envs[0].Env)

	// 业务串不存在 → 404（非 400/500）
	_, err = service.EnvsList(ctx, &GameEnvsListRequest{ID: "no-such-game"})
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	ce, _ := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusNotFound, ce.Code)
}
