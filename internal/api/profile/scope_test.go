package profile

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_UpdateScope_Success(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create admin with admin role
	adminID := createTestAdminWithRole(t, db, "scope_admin", "password123", "admin")

	// Create game
	game := &model.Game{
		GameID: "test-game",
		Name:   "Test Game",
		Status: "dev",
	}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err := svc.UpdateScope(context.Background(), adminID, "test-game", "production")
	assert.NoError(t, err)

	// Verify scope was updated
	admin, err := svcCtx.AdminModel.FindOne(context.Background(), adminID)
	require.NoError(t, err)
	assert.Equal(t, "test-game", admin.LastGameID)
	assert.Equal(t, "production", admin.LastEnv)
}

func TestService_UpdateScope_EmptyGameID(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	adminID := createTestAdminWithRole(t, db, "scope_user", "password123", "admin")

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err := svc.UpdateScope(context.Background(), adminID, "", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gameId 和 env 不能为空")
}

func TestService_UpdateScope_EmptyEnv(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	adminID := createTestAdminWithRole(t, db, "scope_user2", "password123", "admin")

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err := svc.UpdateScope(context.Background(), adminID, "test-game", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gameId 和 env 不能为空")
}

func TestService_UpdateScope_GameNotFound(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	adminID := createTestAdminWithRole(t, db, "scope_user3", "password123", "admin")

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err := svc.UpdateScope(context.Background(), adminID, "nonexistent-game", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "游戏不存在")
}

func TestService_UpdateScope_NonAdminUnauthorized(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create admin with viewer role (not admin)
	adminID := createTestAdminWithRole(t, db, "viewer_user", "password123", "viewer")

	// Create game
	game := &model.Game{
		GameID: "test-game",
		Name:   "Test Game",
		Status: "dev",
	}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err := svc.UpdateScope(context.Background(), adminID, "test-game", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该游戏环境")
}

func TestService_UpdateScope_NonAdminWithGameScope(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create admin with viewer role
	adminID := createTestAdminWithRole(t, db, "scoped_user", "password123", "viewer")

	// Create game
	game := &model.Game{
		GameID: "test-game",
		Name:   "Test Game",
		Status: "dev",
	}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))

	// A game-level scope is not environment authorization.
	adminModel := model.NewAdminModel(db)
	err := adminModel.SetGameScope(context.Background(), adminID, game.ID)
	require.NoError(t, err)

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err = svc.UpdateScope(context.Background(), adminID, "test-game", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该游戏环境")
}

func TestService_UpdateScope_NonAdminWithEnvScope(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create admin with viewer role
	adminID := createTestAdminWithRole(t, db, "env_scoped_user", "password123", "viewer")

	// Create game
	game := &model.Game{
		GameID: "test-game",
		Name:   "Test Game",
		Status: "dev",
	}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))
	require.NoError(t, svcCtx.GameModel.AddEnvBinding(context.Background(), game.GameID, "production", "test", "", ""))

	// Grant env scope to admin
	adminModel := model.NewAdminModel(db)
	err := adminModel.SetGameEnvScope(context.Background(), adminID, game.ID, "production")
	require.NoError(t, err)

	svc := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)
	err = svc.UpdateScope(context.Background(), adminID, "test-game", "production")
	assert.NoError(t, err)
}

// Ensure we import svc to avoid unused import error
var _ = svc.OpsStateStore{}
