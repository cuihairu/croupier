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

// createEnvScopedManager creates an operator with a non-admin role: it holds the games:manage
// function permission (so it passes RequireAnyPermission), but game/env visibility is decided
// by the admin_game_env_scopes grant table (the data source of the gameEnvScopes guard).
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

// seedEnvScope grants an env scope for (adminID, gameID, env).
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

	// Holds games:manage but no game grants at all -> the write path is rejected (the
	// function permission point is no longer the only line of defense)
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

	// A game-level grant (any environment) suffices to add an environment (the new
	// environment does not exist yet and is not in the grant table)
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

	// Only the description changes (name unchanged): the target env prod is within the
	// grant -> passes
	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID: gameID, EnvID: "prod", Name: "prod", Type: "Updated description",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Envs, 2)

	// The target env dev is outside the grant -> rejected
	_, err = service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID: gameID, EnvID: "dev", Type: "Sneaky edit",
	})
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	ce, _ := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusForbidden, ce.Code)

	// Renaming prod -> beta: beta is outside the grant, and a rename is a privilege
	// expansion -> rejected
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

	// Only prod is granted: the listing narrows to prod and dev is not visible
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

	// The frontend authorization view (/profile/games) has no numeric primary key:
	// addressing by business game_id works equivalently
	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")
	service := NewService(svcCtx)
	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{ID: game.GameID})
	require.NoError(t, err)
	require.Len(t, resp.Envs, 1)
	assert.Equal(t, "prod", resp.Envs[0].Env)

	// Business string with no match -> 404 (not 400/500)
	_, err = service.EnvsList(ctx, &GameEnvsListRequest{ID: "no-such-game"})
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expect CodeError, got %T: %v", err, err)
	ce, _ := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusNotFound, ce.Code)
}
