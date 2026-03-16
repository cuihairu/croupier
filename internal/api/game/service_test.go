package game

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	testGameDB          *gorm.DB
	testGameDBOnce      sync.Once
	testGameDBMutex     sync.Mutex
	seedPermMutex       sync.Mutex
	createAdminMutex    sync.Mutex
)

// setupTestDB creates a shared in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	testGameDBMutex.Lock()
	defer testGameDBMutex.Unlock()

	testGameDBOnce.Do(func() {
		var err error
		testGameDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(testGameDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up any existing data before running the test
	testGameDB.Exec("DELETE FROM admin_roles")
	testGameDB.Exec("DELETE FROM admins")
	testGameDB.Exec("DELETE FROM role_permissions")
	testGameDB.Exec("DELETE FROM roles")
	testGameDB.Exec("DELETE FROM permissions")
	testGameDB.Exec("DELETE FROM games")

	return testGameDB
}

// setupTestServiceContext creates a test service context with all necessary dependencies
func setupTestServiceContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	return &svc.ServiceContext{
		DB:                db,
		GameModel:         model.NewGameModel(db),
		AdminModel:        model.NewAdminModel(db),
		RoleModel:         model.NewRoleModel(db),
		PermissionModel:   model.NewPermissionModel(db),
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
	}
}

// seedTestPermissions seeds basic permissions for testing
func seedTestPermissions(t *testing.T, db *gorm.DB) {
	seedPermMutex.Lock()
	defer seedPermMutex.Unlock()

	permissions := []model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "*", Category: "admin"},
		{ID: "games:read", Name: "Games Read", Resource: "games", Action: "read", Category: "games"},
		{ID: "games:manage", Name: "Games Manage", Resource: "games", Action: "*", Category: "games"},
	}

	for _, perm := range permissions {
		err := db.Where("id = ?", perm.ID).FirstOrCreate(&perm).Error
		require.NoError(t, err)
	}

	// Create admin role with admin:all permission
	role := &model.Role{Name: "admin", Description: "Administrator"}
	err := db.Where("name = ?", "admin").FirstOrCreate(role).Error
	require.NoError(t, err)

	for _, perm := range permissions {
		rolePerm := &model.RolePermission{RoleID: role.ID, PermissionID: perm.ID}
		err = db.Where("role_id = ? AND permission_id = ?", role.ID, perm.ID).
			FirstOrCreate(rolePerm).Error
		require.NoError(t, err)
	}
}

// createTestAdminWithRole creates a test admin with a role for testing
// If the admin already exists, it returns the existing admin ID
func createTestAdminWithRole(t *testing.T, db *gorm.DB, username, password, roleName string) uint {
	createAdminMutex.Lock()
	defer createAdminMutex.Unlock()

	adminModel := model.NewAdminModel(db)

	// Try to find existing admin first
	existingAdmin, _ := adminModel.FindByUsername(context.Background(), username)
	if existingAdmin != nil {
		// Admin already exists, return its ID
		return existingAdmin.ID
	}

	// Create admin
	admin := &model.Admin{
		Username: username,
		Nickname: username + " User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, password)
	// If admin already exists (race condition), find it again
	if err != nil {
		existingAdmin, _ := adminModel.FindByUsername(context.Background(), username)
		if existingAdmin != nil {
			return existingAdmin.ID
		}
		require.NoError(t, err)
	}

	// Create role if it doesn't exist
	role := &model.Role{Name: roleName}
	err = db.Where("name = ?", roleName).FirstOrCreate(role).Error
	require.NoError(t, err)

	// Assign role
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	return admin.ID
}

// createTestAdminWithContext creates a test admin and sets context for permission checks
func createTestAdminWithContext(t *testing.T, db *gorm.DB, username, password, roleName string) (context.Context, uint) {
	adminID := createTestAdminWithRole(t, db, username, password, roleName)
	ctx := context.WithValue(context.Background(), "username", username)
	ctx = context.WithValue(ctx, "adminID", adminID)
	return ctx, adminID
}

func TestService_List_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	// Create test games
	gameModel := model.NewGameModel(db)

	game1 := &model.Game{
		Name:      "game1",
		AliasName: "Game One",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game1)
	require.NoError(t, err)

	game2 := &model.Game{
		Name:      "game2",
		AliasName: "Game Two",
		Status:    "dev",
	}
	err = gameModel.Create(context.Background(), game2)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &GamesListRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 2)
	assert.NotEmpty(t, resp.Games)
}

func TestService_List_WithPagination(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	// Create multiple games for pagination
	gameModel := model.NewGameModel(db)
	for i := 1; i <= 15; i++ {
		game := &model.Game{
			Name:      "game_pagination_" + strconv.Itoa(i),
			AliasName: "Game Pagination " + strconv.Itoa(i),
			Status:    "running",
		}
		err := gameModel.Create(context.Background(), game)
		require.NoError(t, err)
	}

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Test first page
	resp1, err := service.List(ctx, &GamesListRequest{
		Page:     1,
		PageSize: 5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp1)
	assert.Len(t, resp1.Games, 5)

	// Test second page
	resp2, err := service.List(ctx, &GamesListRequest{
		Page:     2,
		PageSize: 5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.Len(t, resp2.Games, 5)
}

func TestService_List_WithStatusFilter(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	// Create games with different statuses
	game1 := &model.Game{Name: "game1_status_filter", AliasName: "Game 1 Status Filter", Status: "running"}
	require.NoError(t, gameModel.Create(context.Background(), game1))

	game2 := &model.Game{Name: "game2_status_filter", AliasName: "Game 2 Status Filter", Status: "dev"}
	require.NoError(t, gameModel.Create(context.Background(), game2))

	game3 := &model.Game{Name: "game3_status_filter", AliasName: "Game 3 Status Filter", Status: "test"}
	require.NoError(t, gameModel.Create(context.Background(), game3))

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Test status filter for running games
	resp, err := service.List(ctx, &GamesListRequest{
		Page:     1,
		PageSize: 10,
		Status:   "running",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 1)
}

func TestService_List_Unauthorized(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "username", "unauthorized")

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &GamesListRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:        "testgame_create_success",
		AliasName:   "Test Game Create Success",
		Description: "A test game",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "testgame_create_success", resp.Game.Name)
	assert.Equal(t, "Test Game Create Success", resp.Game.AliasName)
	assert.NotZero(t, resp.Game.ID)
}

func Test_Create_EmptyName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name: "   ", // only whitespace
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func Test_Create_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	// Create initial game
	game1 := &model.Game{
		Name:      "testgame_duplicate_1",
		AliasName: "Test Game Duplicate 1",
		Status:    "dev",
	}
	err := gameModel.Create(context.Background(), game1)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Try to create duplicate
	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:      "testgame_duplicate_1", // duplicate name
		AliasName: "Another Game",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "已存在")
}

func Test_Create_Unauthorized(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "username", "unauthorized")

	service := NewService(svcCtx)

	resp, err := service.Create(ctx, &GameCreateRequest{
		Name:      "newgame",
		AliasName: "New Game",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Detail_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_detail",
		AliasName: "Test Game Detail",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &GameDetailRequest{
		ID: strconv.FormatUint(uint64(game.ID), 10),
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "testgame_detail", resp.Game.Name)
}

func Test_Detail_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &GameDetailRequest{
		ID: "99999",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func Test_Detail_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	tests := []struct {
		name string
		id   string
	}{
		{"empty id", ""},
		{"non-numeric id", "abc"},
		{"zero id", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resp, err := service.Detail(ctx, &GameDetailRequest{
				ID: tt.id,
			})

			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestService_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_update",
		AliasName: "Test Game Update",
		Status:    "dev",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        strconv.FormatUint(uint64(game.ID), 10),
		AliasName: "Updated Game",
		Status:    "running",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Updated Game", resp.Game.AliasName)
	assert.Equal(t, "running", resp.Game.Status)
}

func Test_Update_EmptyUpdate(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_empty_update",
		AliasName: "Test Game Empty Update",
		Status:    "dev",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID: strconv.FormatUint(uint64(game.ID), 10),
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "请提供需要更新的字段")
}

func Test_Update_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game1 := &model.Game{
		Name:      "game1_dup_name",
		AliasName: "Game 1 Dup Name",
		Status:    "dev",
	}
	err := gameModel.Create(context.Background(), game1)
	require.NoError(t, err)

	game2 := &model.Game{
		Name:      "game2_dup_name",
		AliasName: "Game 2 Dup Name",
		Status:    "dev",
	}
	err = gameModel.Create(context.Background(), game2)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Try to rename game2 to game1 (which already exists)
	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:   strconv.FormatUint(uint64(game2.ID), 10),
		Name: "game1_dup_name",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "已存在")
}

func Test_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.Update(ctx, &GameUpdateRequest{
		ID:        "99999",
		AliasName: "Updated",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_delete",
		AliasName: "Test Game Delete",
		Status:    "dev",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	gameID := game.ID

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	err = service.Delete(ctx, &GameDeleteRequest{
		ID: strconv.FormatUint(uint64(gameID), 10),
	})

	assert.NoError(t, err)

	// Verify game is deleted
	_, err = gameModel.FindOne(context.Background(), gameID)
	assert.Error(t, err)
}

func Test_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Delete operation is idempotent - no error even if game doesn't exist
	err := service.Delete(ctx, &GameDeleteRequest{
		ID: "99999",
	})

	// GORM Delete doesn't return an error for non-existent records (idempotent)
	assert.NoError(t, err)
}

func Test_Delete_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	tests := []struct {
		name string
		id   string
	}{
		{"empty id", ""},
		{"non-numeric id", "abc"},
		{"zero id", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := service.Delete(ctx, &GameDeleteRequest{
				ID: tt.id,
			})

			assert.Error(t, err)
		})
	}
}

func TestService_Delete_Unauthorized(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "username", "unauthorized")

	service := NewService(svcCtx)

	err := service.Delete(ctx, &GameDeleteRequest{
		ID: "1",
	})

	assert.Error(t, err)
}

func TestService_EnvsList_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_envs_list",
		AliasName: "Test Game Envs List",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	// Add environments
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "dev", Description: "Development"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(context.Background(), game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{
		ID: strconv.FormatUint(uint64(game.ID), 10),
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 2)
}

func TestService_EnvsList_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvsList(ctx, &GameEnvsListRequest{
		ID: "99999",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_EnvAdd_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_add",
		AliasName: "Test Game Env Add",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "staging",
		Type: "Staging environment",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
}

func Test_EnvAdd_DuplicateEnv(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_add_dup",
		AliasName: "Test Game Env Add Dup",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	// Add initial env
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(context.Background(), game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Try to add duplicate env
	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "prod",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "已存在")
}

func Test_EnvAdd_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	tests := []struct {
		name string
		id   string
	}{
		{"empty id", ""},
		{"non-numeric id", "abc"},
		{"zero id", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
				ID:   tt.id,
				Name: "newenv",
			})

			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestService_EnvUpdate_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_update",
		AliasName: "Test Game Env Update",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	// Add initial env
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(context.Background(), game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "production",
		Type:  "Production Env",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1)
	// The env name should be updated to "production" and description to "Production Env"
	assert.Equal(t, "production", resp.Envs[0].Env)
	assert.Equal(t, "Production Env", resp.Envs[0].Description)
}

func Test_EnvUpdate_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_update_not_found",
		AliasName: "Test Game Env Update Not Found",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "nonexistent",
		Name:  "new name",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "不存在")
}

func Test_EnvDelete_Success(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_delete",
		AliasName: "Test Game Env Delete",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	// Add initial envs
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "dev", Description: "Development"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(context.Background(), game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Envs, 1) // One env should remain (dev)
}

func Test_EnvDelete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_delete_not_found",
		AliasName: "Test Game Env Delete Not Found",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "nonexistent",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "不存在")
}

func Test_EnvDelete_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	tests := []struct {
		name string
		id   string
		env  string
	}{
		{"empty id", "", "prod"},
		{"non-numeric id", "abc", "prod"},
		{"zero id", "0", "prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
				ID:    tt.id,
				EnvID: tt.env,
			})

			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func Test_EnvDelete_MissingEnv(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_delete_missing",
		AliasName: "Test Game Env Delete Missing",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Try to delete an env that doesn't exist
	resp, err := service.EnvDelete(ctx, &GameEnvDeleteRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "nonexistent",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "不存在")
}

func TestService_EnvAdd_MissingEnv(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_add_missing",
		AliasName: "Test Game Env Add Missing",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Missing env field should fail validation
	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Type:  "some type",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_EnvAdd_EmptyEnv(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_add_empty",
		AliasName: "Test Game Env Add Empty",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	resp, err := service.EnvAdd(ctx, &GameEnvAddRequest{
		ID:   strconv.FormatUint(uint64(game.ID), 10),
		Name: "   ", // only whitespace
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_EnvUpdate_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)

	gameModel := model.NewGameModel(db)

	game := &model.Game{
		Name:      "testgame_env_update_dup",
		AliasName: "Test Game Env Update Dup",
		Status:    "running",
	}
	err := gameModel.Create(context.Background(), game)
	require.NoError(t, err)

	// Add initial envs
	envs := []model.GameEnv{
		{Env: "prod", Description: "Production"},
		{Env: "dev", Description: "Development"},
	}
	err = game.SetEnvs(envs)
	require.NoError(t, err)
	err = gameModel.Update(context.Background(), game.ID, map[string]interface{}{"envs": game.Envs})
	require.NoError(t, err)

	ctx, _ := createTestAdminWithContext(t, db, "testadmin", "password123", "admin")

	service := NewService(svcCtx)

	// Try to rename prod to dev (which already exists)
	resp, err := service.EnvUpdate(ctx, &GameEnvUpdateRequest{
		ID:    strconv.FormatUint(uint64(game.ID), 10),
		EnvID: "prod",
		Name:  "dev",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "已存在")
}
