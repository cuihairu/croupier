package profile

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	testProfileDB      *gorm.DB
	testProfileDBOnce  sync.Once
	testProfileDBMutex sync.Mutex
)

// setupTestDB creates a shared in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	testProfileDBMutex.Lock()
	defer testProfileDBMutex.Unlock()

	testProfileDBOnce.Do(func() {
		var err error
		testProfileDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(testProfileDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up any existing data before running the test
	testProfileDB.Exec("DELETE FROM admin_roles")
	testProfileDB.Exec("DELETE FROM admins")
	testProfileDB.Exec("DELETE FROM roles")
	testProfileDB.Exec("DELETE FROM role_permissions")
	testProfileDB.Exec("DELETE FROM permissions")
	testProfileDB.Exec("DELETE FROM admin_game_scopes")
	testProfileDB.Exec("DELETE FROM admin_game_env_scopes")
	testProfileDB.Exec("DELETE FROM game_envs")
	testProfileDB.Exec("DELETE FROM games")

	return testProfileDB
}

// setupTestServiceContext creates a test service context with all necessary dependencies
func setupTestServiceContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	return &svc.ServiceContext{
		DB:              db,
		AdminModel:      model.NewAdminModel(db),
		GameModel:       model.NewGameModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
		Cache:           nullCache,
		CacheHelper:     cacheHelper,
	}
}

// createTestAdminWithRole creates a test admin with a role for testing
func createTestAdminWithRole(t *testing.T, db *gorm.DB, username, password, roleName string) uint {
	adminModel := model.NewAdminModel(db)

	// Create admin
	admin := &model.Admin{
		Username: username,
		Nickname: username + " User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, password)
	require.NoError(t, err)

	// Create role if it doesn't exist
	role := &model.Role{Name: roleName}
	err = db.Where("name = ?", roleName).FirstOrCreate(role).Error
	require.NoError(t, err)

	// Assign role
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	return admin.ID
}

// createProfileTestGame persists both the legacy UI metadata and the
// authoritative game_envs records used by scope resolution.
func createProfileTestGame(t *testing.T, gameModel *model.GameModel, game *model.Game) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, gameModel.Create(ctx, game))

	envs, err := game.GetEnvs()
	require.NoError(t, err)
	for _, env := range envs {
		require.NoError(t, gameModel.AddEnvBinding(ctx, game.GameID, env.Env, "test", env.Description, env.Color))
	}
}

func TestService_GetProfile_Success(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "profileuser",
		Nickname: "Profile User",
		Email:    "profile@example.com",
		Phone:    "1234567890",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetProfile(context.Background(), "profileuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "profileuser", resp.ProfileInfo.Username)
	assert.Equal(t, "Profile User", resp.ProfileInfo.Nickname)
	assert.NotEmpty(t, resp.ProfileInfo.Roles)
}

func TestService_GetProfile_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	resp, err := service.GetProfile(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_GetUserGames_AdminWithScopes(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	adminID := createTestAdminWithRole(t, db, "scopeduser", "password123", "admin")

	// Create a game
	game1 := &model.Game{
		Name:      "game1_scoped",
		AliasName: "Game One Scoped",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game1.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game1)

	// Assign game scope to admin
	err = adminModel.SetGameEnvScope(context.Background(), adminID, game1.ID, "prod")
	require.NoError(t, err)

	resp, err := service.GetUserGames(context.Background(), "scopeduser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Games, 1)
	assert.Equal(t, "game1_scoped", resp.Games[0].GameId)
}

func TestService_GetUserGames_AdminNoScopes(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "noscopeuser", "password123", "admin")

	// Create a game
	game1 := &model.Game{
		Name:      "game2_noscope",
		AliasName: "Game Two NoScope",
		Color:     "#00ff00",
		Status:    "running",
	}
	err := game1.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game1)

	resp, err := service.GetUserGames(context.Background(), "noscopeuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Games), 1)
}

func TestService_GetUserGames_UserNotFound(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	resp, err := service.GetUserGames(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_UpdateProfile_Success(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "updateuser",
		Nickname: "Update User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "updateuser", &ProfileUpdateRequest{
		Nickname: "Updated User",
		Email:    "updated@example.com",
		Phone:    "9876543210",
		Avatar:   "https://example.com/avatar.png",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	resp, err := service.UpdateProfile(context.Background(), "nonexistent", &ProfileUpdateRequest{
		Nickname: "Test",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_ChangePassword_Success(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "changepassuser",
		Nickname: "Change Pass User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "oldpassword123")
	require.NoError(t, err)

	resp, err := service.ChangePassword(context.Background(), "changepassuser", &ChangePasswordRequest{
		OldPassword: "oldpassword123",
		NewPassword: "newpassword123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_ChangePassword_WrongOldPassword(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "wrongpassuser",
		Nickname: "Wrong Pass User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "correctpassword123")
	require.NoError(t, err)

	resp, err := service.ChangePassword(context.Background(), "wrongpassuser", &ChangePasswordRequest{
		OldPassword: "wrongpassword",
		NewPassword: "newpassword123",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "旧密码错误")
}

func TestService_ChangePassword_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	resp, err := service.ChangePassword(context.Background(), "nonexistent", &ChangePasswordRequest{
		OldPassword: "oldpassword",
		NewPassword: "newpassword",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "旧密码错误")
}

func TestService_GetPermissions_Success(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	createTestAdminWithRole(t, db, "permsuser", "password123", "admin")

	resp, err := service.GetPermissions(context.Background(), "permsuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Roles)
	assert.True(t, resp.Admin)
	assert.NotEmpty(t, resp.PermissionIDs)
}

func TestService_GetPermissions_UserNotFound(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	resp, err := service.GetPermissions(context.Background(), "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "用户不存在")
}

func TestService_GetPermissions_NoRoles(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	admin := &model.Admin{
		Username: "norolesuser",
		Nickname: "No Roles User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.GetPermissions(context.Background(), "norolesuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Roles)
	assert.False(t, resp.Admin)
}

func TestService_GetProfile_MultipleRoles(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	admin := &model.Admin{
		Username: "multiroleuser",
		Nickname: "Multi Role User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	// Create multiple roles
	role1 := &model.Role{Name: "admin"}
	err = db.Create(role1).Error
	require.NoError(t, err)
	role2 := &model.Role{Name: "editor"}
	err = db.Create(role2).Error
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role1.ID)
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role2.ID)
	require.NoError(t, err)

	resp, err := service.GetProfile(context.Background(), "multiroleuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.ProfileInfo.Roles, 2)
}

func TestService_GetUserGames_SortsGamesByName(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "sortuser", "password123", "admin")

	// Create multiple games
	game2 := &model.Game{
		Name:      "game2_sort",
		AliasName: "Game B",
		Color:     "#00ff00",
		Status:    "running",
	}
	game1 := &model.Game{
		Name:      "game1_sort",
		AliasName: "Game A",
		Color:     "#ff0000",
		Status:    "running",
	}
	for _, game := range []*model.Game{game2, game1} {
		err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
		require.NoError(t, err)
		createProfileTestGame(t, gameModel, game)
	}

	resp, err := service.GetUserGames(context.Background(), "sortuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Games returned should include both games
	assert.GreaterOrEqual(t, len(resp.Games), 2)
	// Both games should be present
	gameNames := make(map[string]bool)
	for _, g := range resp.Games {
		gameNames[g.GameName] = true
	}
	assert.True(t, gameNames["Game A"])
	assert.True(t, gameNames["Game B"])
}

func TestService_GetUserGames_GameWithNoName(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "nonameuser", "password123", "admin")

	// Create a game with empty name. GameID is the canonical identifier and
	// BeforeCreate auto-derives it, so the game is still listed and falls
	// back to the derived GameID as its display name.
	game := &model.Game{
		Name:      "",
		AliasName: "",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "nonameuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	require.Len(t, resp.Games, 1)
	assert.Equal(t, "game", resp.Games[0].GameId)
	assert.Equal(t, "game", resp.Games[0].GameName)
}

func TestService_UpdateProfile_WithAllFields(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "allfieldsuser",
		Nickname: "All Fields User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "allfieldsuser", &ProfileUpdateRequest{
		Nickname: "All Fields Updated",
		Email:    "allfields@example.com",
		Phone:    "1111111111",
		Avatar:   "https://example.com/avatar.png",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_GetUserGames_AdminWithSuperAdminRole(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	// Create admin with super_admin role
	admin := &model.Admin{
		Username: "superadminuser",
		Nickname: "Super Admin User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "super_admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	// Create multiple games
	game1 := &model.Game{
		Name:      "game1_super",
		AliasName: "Game One Super",
		Color:     "#ff0000",
		Status:    "running",
	}
	game2 := &model.Game{
		Name:      "game2_super",
		AliasName: "Game Two Super",
		Color:     "#00ff00",
		Status:    "running",
	}
	for _, game := range []*model.Game{game1, game2} {
		err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
		require.NoError(t, err)
		createProfileTestGame(t, gameModel, game)
	}

	resp, err := service.GetUserGames(context.Background(), "superadminuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Games), 2)
}

func TestService_GetUserGames_GameWithEnvs(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "envsuser", "password123", "admin")

	// Create game with multiple environments
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "staging", Description: "Staging"},
		{Env: "dev", Description: "Development"},
	}
	game := &model.Game{
		Name:      "envgame_test",
		AliasName: "Env Game",
		Color:     "#0000ff",
		Status:    "running",
	}
	err := game.SetEnvs(envs)
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "envsuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if len(resp.Games) > 0 {
		assert.Len(t, resp.Games[0].Envs, 3)
	}
}

func TestService_GetUserGames_AdminWithViewerRole(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "vieweruser", "password123", "viewer")

	// Create a game
	game1 := &model.Game{
		Name:      "game1_viewer",
		AliasName: "Game One Viewer",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game1.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game1)

	resp, err := service.GetUserGames(context.Background(), "vieweruser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Viewer role with no scope entries sees 0 games (must have explicit scope)
	assert.Equal(t, 0, len(resp.Games))
}

func TestService_GetProfile_EmptyEmail(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	admin := &model.Admin{
		Username: "noemailuser",
		Nickname: "No Email User",
		Email:    "",
		Phone:    "1234567890",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetProfile(context.Background(), "noemailuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "noemailuser", resp.ProfileInfo.Username)
	assert.Empty(t, resp.ProfileInfo.Email)
}

func TestService_ChangePassword_EmptyNewPassword(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "emptypassuser",
		Nickname: "Empty Pass User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.ChangePassword(context.Background(), "emptypassuser", &ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "",
	})

	// Service should allow empty password (no validation in current implementation)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_OnlyNickname(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "nickonlyuser",
		Nickname: "Nick Only User",
		Email:    "nick@example.com",
		Phone:    "1234567890",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "nickonlyuser", &ProfileUpdateRequest{
		Nickname: "Updated Nickname",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_OnlyEmail(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "emailonlyuser",
		Nickname: "Email Only User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "emailonlyuser", &ProfileUpdateRequest{
		Email: "newemail@example.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_OnlyPhone(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "phoneonlyuser",
		Nickname: "Phone Only User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "phoneonlyuser", &ProfileUpdateRequest{
		Phone: "9876543210",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_OnlyAvatar(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "avataronlyuser",
		Nickname: "Avatar Only User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "avataronlyuser", &ProfileUpdateRequest{
		Avatar: "https://example.com/new-avatar.png",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_GetPermissions_WithAdminRole(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	admin := &model.Admin{
		Username: "adminpermsuser",
		Nickname: "Admin Perms User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetPermissions(context.Background(), "adminpermsuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Admin)
	assert.Contains(t, resp.Roles, "admin")
	assert.Contains(t, resp.PermissionIDs, "admin")
	assert.Contains(t, resp.PermissionIDs, "*")
}

func TestService_GetPermissions_WithSuperAdminRole(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	admin := &model.Admin{
		Username: "superadminpermsuser",
		Nickname: "SuperAdmin Perms User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "super_admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetPermissions(context.Background(), "superadminpermsuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Admin)
	assert.Contains(t, resp.Roles, "super_admin")
	assert.Contains(t, resp.PermissionIDs, "super_admin")
	assert.Contains(t, resp.PermissionIDs, "admin")
	assert.Contains(t, resp.PermissionIDs, "*")
}

func TestService_GetPermissions_WithCustomRole(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	admin := &model.Admin{
		Username: "customroleuser",
		Nickname: "Custom Role User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "custom_operator"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetPermissions(context.Background(), "customroleuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Admin)
	assert.Contains(t, resp.Roles, "custom_operator")
	assert.Contains(t, resp.PermissionIDs, "custom_operator")
}

func TestService_GetProfile_WithEmailAndPhone(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	admin := &model.Admin{
		Username: "emailphoneuser",
		Nickname: "Email Phone User",
		Email:    "emailphone@example.com",
		Phone:    "5555555555",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetProfile(context.Background(), "emailphoneuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "emailphoneuser", resp.ProfileInfo.Username)
	assert.Equal(t, "emailphone@example.com", resp.ProfileInfo.Email)
	assert.Equal(t, "5555555555", resp.ProfileInfo.Phone)
}

func TestService_GetProfile_WithAvatar(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	admin := &model.Admin{
		Username: "avataruser",
		Nickname: "Avatar User",
		Avatar:   "https://example.com/avatar.png",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "admin"}
	err = db.Create(role).Error
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	resp, err := service.GetProfile(context.Background(), "avataruser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "https://example.com/avatar.png", resp.ProfileInfo.Avatar)
}

func TestService_GetUserGames_WithMultipleScopes(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	adminID := createTestAdminWithRole(t, db, "multiscopeuser", "password123", "operator")

	// Create multiple games
	game1 := &model.Game{
		Name:      "game1_multi",
		AliasName: "Game One Multi",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game1.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game1)

	game2 := &model.Game{
		Name:      "game2_multi",
		AliasName: "Game Two Multi",
		Color:     "#00ff00",
		Status:    "running",
	}
	err = game2.SetEnvs([]model.GameEnv{{Env: "staging"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game2)

	// Assign both games to admin
	err = adminModel.SetGameEnvScope(context.Background(), adminID, game1.ID, "prod")
	require.NoError(t, err)
	err = adminModel.SetGameEnvScope(context.Background(), adminID, game2.ID, "staging")
	require.NoError(t, err)

	resp, err := service.GetUserGames(context.Background(), "multiscopeuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Operator role is not admin/super_admin, so only scoped games
	assert.GreaterOrEqual(t, len(resp.Games), 2)
}

func TestService_ChangePassword_SamePassword(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "samepassuser",
		Nickname: "Same Pass User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.ChangePassword(context.Background(), "samepassuser", &ChangePasswordRequest{
		OldPassword: "password123",
		NewPassword: "password123",
	})

	// Should allow changing to same password
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_UpdateProfile_EmptyFields(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "emptyfieldsuser",
		Nickname: "Empty Fields User",
		Email:    "old@example.com",
		Phone:    "1111111111",
		Avatar:   "https://example.com/old.png",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	resp, err := service.UpdateProfile(context.Background(), "emptyfieldsuser", &ProfileUpdateRequest{
		Nickname: "",
		Email:    "",
		Phone:    "",
		Avatar:   "",
	})

	// Should allow updating to empty strings
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ok)
}

func TestService_GetUserGames_GameStatusStopped(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "stoppedgameuser", "password123", "admin")

	// Create a stopped game
	game := &model.Game{
		Name:      "stoppedgame",
		AliasName: "Stopped Game",
		Color:     "#808080",
		Status:    "stopped",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "stoppedgameuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Admin users should see all games including stopped ones
	assert.GreaterOrEqual(t, len(resp.Games), 1)
}

func TestService_GetPermissions_MultipleRolesIncludingAdmin(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, roleModel)

	admin := &model.Admin{
		Username: "multiadminuser",
		Nickname: "Multi Admin User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	// Create multiple roles including admin
	role1 := &model.Role{Name: "admin"}
	err = db.Create(role1).Error
	require.NoError(t, err)
	role2 := &model.Role{Name: "editor"}
	err = db.Create(role2).Error
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role1.ID)
	require.NoError(t, err)
	err = adminModel.AssignRole(context.Background(), admin.ID, role2.ID)
	require.NoError(t, err)

	resp, err := service.GetPermissions(context.Background(), "multiadminuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Admin) // Should be true because has admin role
	assert.Len(t, resp.Roles, 2)
	assert.Contains(t, resp.PermissionIDs, "*") // Admin role adds wildcard
}

func TestService_GetUserGames_WithDeletedGameScope(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	adminID := createTestAdminWithRole(t, db, "delscopeuser", "password123", "operator")

	// Assign a game scope to a game that doesn't exist
	err := adminModel.SetGameScope(context.Background(), adminID, 99999)
	require.NoError(t, err)

	resp, err := service.GetUserGames(context.Background(), "delscopeuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// No games should be returned since the scoped game doesn't exist
	assert.Equal(t, 0, len(resp.Games))
}

func TestService_GetUserGames_GameWithWhitespaceName(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "whitespaceuser2", "password123", "admin")

	// Create a game with whitespace-only name (should be filtered)
	game := &model.Game{
		Name:      "   ",
		AliasName: "Whitespace Game",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "whitespaceuser2")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Games with whitespace-only names should be filtered out
	for _, g := range resp.Games {
		assert.NotEqual(t, "   ", g.GameId)
	}
}

func TestService_GetUserGames_GameWithEmptyAliasName(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "noaliasuser", "password123", "admin")

	// Create a game with empty alias name (should use Name as fallback)
	game := &model.Game{
		Name:      "noalias_game",
		AliasName: "",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "noaliasuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should find the game with Name used as GameName
	found := false
	for _, g := range resp.Games {
		if g.GameId == "noalias_game" {
			assert.Equal(t, "noalias_game", g.GameName) // Falls back to Name
			found = true
			break
		}
	}
	assert.True(t, found, "Should find the game with empty alias name")
}

func TestService_GetUserGames_GameEnvWithWhitespace(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "envwhitespaceuser", "password123", "admin")

	// Create a game with environments that have whitespace
	envs := []model.GameEnv{
		{Env: "  prod  ", Description: "Production"},
		{Env: "  ", Description: "Empty Env"},
		{Env: "staging", Description: "Staging"},
	}
	game := &model.Game{
		Name:      "envws_game",
		AliasName: "Env Whitespace Game",
		Color:     "#0000ff",
		Status:    "running",
	}
	err := game.SetEnvs(envs)
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "envwhitespaceuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should find the game with trimmed env names
	found := false
	for _, g := range resp.Games {
		if g.GameId == "envws_game" {
			// Whitespace envs should be trimmed, empty envs should be filtered
			assert.NotContains(t, g.Envs, "  ")
			assert.NotContains(t, g.Envs, "  prod  ")
			if len(g.Envs) > 0 {
				assert.Contains(t, g.Envs, "prod") // Should have trimmed env
			}
			found = true
			break
		}
	}
	assert.True(t, found, "Should find the game with whitespace envs")
}

func TestService_GetUserGames_DuplicateGames(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "dupuser", "password123", "admin")

	// Create a game
	game := &model.Game{
		Name:      "dupgame",
		AliasName: "Duplicate Game",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "dupuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should deduplicate games by name
	count := 0
	for _, g := range resp.Games {
		if g.GameId == "dupgame" {
			count++
		}
	}
	assert.Equal(t, 1, count, "Should only have one instance of dupgame")
}

func TestService_ChangePassword_ValidatePasswordError(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	adminModel := model.NewAdminModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, svcCtx.GameModel, permSvc)

	admin := &model.Admin{
		Username: "validateerroruser",
		Nickname: "Validate Error User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	// Try to change with wrong old password
	resp, err := service.ChangePassword(context.Background(), "validateerroruser", &ChangePasswordRequest{
		OldPassword: "wrongpassword",
		NewPassword: "newpassword123",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "旧密码错误")
}

func TestService_GetUserGames_NoGamesInSystem(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "nogamesuser", "password123", "operator")

	// Don't create any games

	resp, err := service.GetUserGames(context.Background(), "nogamesuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// No games in system
	assert.Empty(t, resp.Games)
}

func TestService_GetUserGames_GameWithNilColor(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "nocoloruser", "password123", "admin")

	// Create a game with empty color
	game := &model.Game{
		Name:      "nocolorgame",
		AliasName: "No Color Game",
		Color:     "",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "nocoloruser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should find the game with empty color
	found := false
	for _, g := range resp.Games {
		if g.GameId == "nocolorgame" {
			assert.Equal(t, "", g.Color)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find the game with empty color")
}

func TestService_GetUserGames_GameWithWhitespaceAlias(t *testing.T) {
	db := setupTestDB(t)

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	permSvc := model.NewRoleModel(db)

	service := NewService(adminModel, gameModel, permSvc)

	createTestAdminWithRole(t, db, "wsaliasuser", "password123", "admin")

	// Create a game with whitespace-only alias name (should use Name as fallback)
	game := &model.Game{
		Name:      "wsaliasgame",
		AliasName: "   ",
		Color:     "#ff0000",
		Status:    "running",
	}
	err := game.SetEnvs([]model.GameEnv{{Env: "prod"}})
	require.NoError(t, err)
	createProfileTestGame(t, gameModel, game)

	resp, err := service.GetUserGames(context.Background(), "wsaliasuser")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should find the game with Name used as GameName since AliasName is whitespace
	found := false
	for _, g := range resp.Games {
		if g.GameId == "wsaliasgame" {
			assert.Equal(t, "wsaliasgame", g.GameName)
			found = true
			break
		}
	}
	assert.True(t, found, "Should find the game and fall back to Name for GameName")
}

// TestService_ResolveLastLoginAt_WithTimestamp tests with existing timestamp
func TestService_ResolveLastLoginAt_WithTimestamp(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)

	now := "2024-01-15T10:30:00Z"
	ts, err := parseTimestampPtr(now)
	require.NoError(t, err)

	result := service.resolveLastLoginAt("admin", ts)
	assert.Equal(t, now, result)
}

// TestService_ResolveLastLoginAt_NilTimestamp tests with nil timestamp
func TestService_ResolveLastLoginAt_NilTimestamp(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)

	result := service.resolveLastLoginAt("admin", nil)
	// Should return empty string when no ops store
	assert.Equal(t, "", result)
}

// TestService_ResolveLastLoginAt_ZeroTimestamp tests with zero timestamp
func TestService_ResolveLastLoginAt_ZeroTimestamp(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel)

	var ts time.Time
	result := service.resolveLastLoginAt("admin", &ts)
	// Should return empty string when timestamp is zero
	assert.Equal(t, "", result)
}

// parseTimestampPtr is a helper function for testing
func parseTimestampPtr(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// TestService_ResolveLastLoginAt_WithOpsStore tests with opsStore containing audit entries
func TestService_ResolveLastLoginAt_WithOpsStore(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with audit entries
	store := svc.NewOpsStateStore("")
	loginTime := time.Now()
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "admin", Action: "login", Result: "success", CreatedAt: loginTime},
			{UserID: "other", Action: "login", Result: "success", CreatedAt: loginTime},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.NotEmpty(t, result, "Should find login entry for admin")
}

// TestService_ResolveLastLoginAt_NoMatchingEntry tests with no matching audit entry
func TestService_ResolveLastLoginAt_NoMatchingEntry(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with entries for different user
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "other", Action: "login", Result: "success", CreatedAt: time.Now()},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.Empty(t, result, "Should return empty when no matching entry found")
}

// TestService_ResolveLastLoginAt_NonLoginAction tests with non-login action
func TestService_ResolveLastLoginAt_NonLoginAction(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with non-login action
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "admin", Action: "logout", Result: "success", CreatedAt: time.Now()},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.Empty(t, result, "Should return empty for non-login action")
}

// TestService_ResolveLastLoginAt_FailedLogin tests with failed login entry
func TestService_ResolveLastLoginAt_FailedLogin(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with failed login
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "admin", Action: "login", Result: "failed", CreatedAt: time.Now()},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.Empty(t, result, "Should return empty for failed login")
}

// TestService_ResolveLastLoginAt_ZeroCreatedAt tests with zero CreatedAt
func TestService_ResolveLastLoginAt_ZeroCreatedAt(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with entry having zero CreatedAt
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "admin", Action: "login", Result: "success", CreatedAt: time.Time{}},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.Empty(t, result, "Should return empty when CreatedAt is zero")
}

// TestService_ResolveLastLoginAt_CaseInsensitiveUsername tests case-insensitive username matching
func TestService_ResolveLastLoginAt_CaseInsensitiveUsername(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with different case username
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "Admin", Action: "login", Result: "success", CreatedAt: time.Now()},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.NotEmpty(t, result, "Should match username case-insensitively")
}

// TestService_ResolveLastLoginAt_AuthLoginAction tests with "auth.login" action
func TestService_ResolveLastLoginAt_AuthLoginAction(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)

	// Create ops store with "auth.login" action
	store := svc.NewOpsStateStore("")
	_, _ = store.Update(func(st *svc.OpsState) {
		st.Audit.Entries = []svc.OpsAuditEntry{
			{UserID: "admin", Action: "auth.login", Result: "success", CreatedAt: time.Now()},
		}
	})
	service := NewService(svcCtx.AdminModel, svcCtx.GameModel, svcCtx.RoleModel, store)

	result := service.resolveLastLoginAt("admin", nil)
	assert.NotEmpty(t, result, "Should recognize 'auth.login' action")
}

func TestHasProfileAdminRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []model.Role
		expected bool
	}{
		{"nil roles", nil, false},
		{"empty roles", []model.Role{}, false},
		{"admin role", []model.Role{{Name: "admin"}}, true},
		{"super_admin role", []model.Role{{Name: "super_admin"}}, true},
		{"Admin role uppercase", []model.Role{{Name: "Admin"}}, true},
		{"ADMIN role uppercase", []model.Role{{Name: "ADMIN"}}, true},
		{"admin with spaces", []model.Role{{Name: "  admin  "}}, true},
		{"super_admin with spaces", []model.Role{{Name: "  super_admin  "}}, true},
		{"operator role", []model.Role{{Name: "operator"}}, false},
		{"viewer role", []model.Role{{Name: "viewer"}}, false},
		{"multiple roles with admin", []model.Role{{Name: "viewer"}, {Name: "admin"}}, true},
		{"multiple roles without admin", []model.Role{{Name: "viewer"}, {Name: "operator"}}, false},
		{"partial match", []model.Role{{Name: "admin_read"}}, false},
		{"admin substring", []model.Role{{Name: "myadmin"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasProfileAdminRole(tt.roles)
			assert.Equal(t, tt.expected, result)
		})
	}
}
