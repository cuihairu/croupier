package svc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Helper function to create an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	// Use a unique in-memory database for each test to avoid conflicts in parallel tests
	// Mode=memory creates a separate in-memory database for each connection
	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// Helper function to create a test service context
func setupTestServiceContext(t *testing.T) *ServiceContext {
	db := setupTestDB(t)

	// Run auto migration
	require.NoError(t, autoMigrate(db))

	// Create a minimal config
	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: ":memory:",
		},
		Cache: config.CacheConfig{
			Type: "local",
		},
		Storage: config.StorageConfig{
			Driver: "",
		},
		BootstrapData: config.BootstrapDataConfig{
			BaseDir: "",
		},
		Auth: config.AuthConfig{
			UsersConfig: "",
			GamesConfig: "",
		},
		AgentDispatch: config.AgentDispatchConfig{
			JobRoutingDir: "",
			JobRoutingTTL: "",
			ToAgentTLS: config.TLSClientConfig{
				Enabled: false,
			},
		},
	}

	// Create cache store
	cacheStore := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(cacheStore)

	// Create admin manager with temp directory
	tmpDir := t.TempDir()

	// Create default admin file
	adminData := `[{"username":"admin","password":"admin123","roles":["admin"],"nickname":"Administrator","email":"admin@test.com","status":1}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)

	// Create default roles file
	rolesData := `[{"code":"admin","name":"Admin","description":"Administrator","level":1,"permissions":["*"]}]`
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesData), 0o644)
	require.NoError(t, err)

	// Create default permissions file
	permsData := `[{"code":"*","name":"All Permissions","description":"All permissions","category":"admin","module":"*"}]`
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte(permsData), 0o644)
	require.NoError(t, err)

	// Initialize models manually for testing
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)
	gameModel := model.NewGameModel(db)
	playerModel := model.NewPlayerModel(db)
	profileModel := model.NewProfileModel(db)
	functionModel := model.NewFunctionModel(db)
	termDictModel := model.NewTermDictionaryModel(db)
	nodeModel := model.NewNodeModel(db)
	ticketModel := model.NewTicketModel(db)
	messageModel := model.NewMessageModel(db)
	alertModel := model.NewAlertModel(db)
	backupModel := model.NewBackupModel(db)
	faqModel := model.NewFAQModel(db)
	feedbackModel := model.NewFeedbackModel(db)
	entityModel := model.NewEntityModel(db)
	behaviorModel := model.NewBehaviorModel(db)
	retentionModel := model.NewRetentionModel(db)
	paymentsModel := model.NewPaymentsModel(db)
	rateLimitModel := model.NewRateLimitModel(db)
	supportModel := model.NewSupportModel(db)
	certificateModel := model.NewCertificateModel(db)
	configVersionModel := model.NewConfigVersionModel(db)
	workspaceConfigModel := model.NewWorkspaceConfigModel(db)
	agentSessionModel := reg.NewAgentSessionModel(db)

	adminManager := NewAdminManager(tmpDir)
	err = adminManager.Initialize()
	require.NoError(t, err)

	opsStateStore := NewOpsStateStore(tmpDir)

	ctx := &ServiceContext{
		Config:               cfg,
		DB:                   db,
		Cache:                cacheStore,
		CacheHelper:          cacheHelper,
		AdminModel:           adminModel,
		RoleModel:            roleModel,
		PermissionModel:      permissionModel,
		GameModel:            gameModel,
		PlayerModel:          playerModel,
		ProfileModel:         profileModel,
		FunctionModel:        functionModel,
		TermDictModel:        termDictModel,
		NodeModel:            nodeModel,
		TicketModel:          ticketModel,
		MessageModel:         messageModel,
		AlertModel:           alertModel,
		BackupModel:          backupModel,
		FAQModel:             faqModel,
		FeedbackModel:        feedbackModel,
		EntityModel:          entityModel,
		BehaviorModel:        behaviorModel,
		RetentionModel:       retentionModel,
		PaymentsModel:        paymentsModel,
		RateLimitModel:       rateLimitModel,
		SupportModel:         supportModel,
		CertificateModel:     certificateModel,
		ConfigVersionModel:   configVersionModel,
		WorkspaceConfigModel: workspaceConfigModel,
		AgentSessionModel:    agentSessionModel,
		AdminManager:         adminManager,
		OpsStateStore:        opsStateStore,
		StartTime:            time.Now(),
		AnalyticsFiltersLock: &sync.RWMutex{},
		MetricsStore:         reg.NewMetricsStore(),
		SystemInfoCache:      reg.NewSystemInfoCache(),
	}

	// Initialize registry store and dispatcher
	ctx.RegistryStore = reg.NewStoreWithDB(ctx.DB)
	ctx.Dispatcher = dispatch.NewDispatcher(ctx.RegistryStore)

	return ctx
}

// ============================================================================
// Tests for Options
// ============================================================================

func TestWithRegistryStore(t *testing.T) {
	t.Parallel()

	store := reg.NewStore()
	opt := WithRegistryStore(store)

	cfg := config.Config{}
	ctx := &ServiceContext{Config: cfg}
	opt(ctx)

	assert.Same(t, store, ctx.RegistryStore)
}

func TestWithRegistryStore_Nil(t *testing.T) {
	t.Parallel()

	opt := WithRegistryStore(nil)
	cfg := config.Config{}
	ctx := &ServiceContext{Config: cfg, RegistryStore: reg.NewStore()}
	oldStore := ctx.RegistryStore

	opt(ctx)

	// Should not replace existing store with nil
	assert.Same(t, oldStore, ctx.RegistryStore)
}

func TestWithDispatcher(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.NewDispatcher(reg.NewStore())
	opt := WithDispatcher(dispatcher)

	cfg := config.Config{}
	ctx := &ServiceContext{Config: cfg}
	opt(ctx)

	assert.Same(t, dispatcher, ctx.Dispatcher)
}

func TestWithDispatcher_Nil(t *testing.T) {
	t.Parallel()

	opt := WithDispatcher(nil)
	cfg := config.Config{}
	existingDispatcher := dispatch.NewDispatcher(reg.NewStore())
	ctx := &ServiceContext{Config: cfg, Dispatcher: existingDispatcher}
	oldDispatcher := ctx.Dispatcher

	opt(ctx)

	// Should not replace existing dispatcher with nil
	assert.Same(t, oldDispatcher, ctx.Dispatcher)
}

// ============================================================================
// Tests for Cache Layer
// ============================================================================

func TestCachedFetch_NoCache(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{CacheHelper: nil}
	called := false
	loader := func() (interface{}, error) {
		called = true
		return "test", nil
	}

	var result string
	err := ctx.cachedFetch(context.Background(), "", &result, loader)

	assert.True(t, called, "loader should be called when no cache")
	assert.NoError(t, err)
	assert.Equal(t, "test", result)
}

func TestCachedFetch_EmptyKey(t *testing.T) {
	t.Parallel()

	cacheStore := cache.NewNullCache()
	ctx := &ServiceContext{
		Cache:       cacheStore,
		CacheHelper: cache.NewCacheHelper(cacheStore),
	}

	called := false
	loader := func() (interface{}, error) {
		called = true
		return "test", nil
	}

	var result string
	err := ctx.cachedFetch(context.Background(), "", &result, loader)

	assert.True(t, called, "loader should be called when key is empty")
	assert.NoError(t, err)
	assert.Equal(t, "test", result)
}

func TestGetAdminCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create an admin
	admin := &model.Admin{
		Username: "testuser",
		Nickname: "Test User",
		Email:    "test@example.com",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password123")
	require.NoError(t, err)

	// First call should load from DB
	cachedAdmin, err := ctx.GetAdminCached(bg, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, "testuser", cachedAdmin.Username)

	// Update the DB directly
	err = ctx.AdminModel.Update(bg, admin.ID, map[string]interface{}{"nickname": "Updated"})
	require.NoError(t, err)

	// Invalidate cache
	ctx.InvalidateAdminCache(bg, admin.ID, "testuser")

	// Load again should get updated data
	cachedAdmin2, err := ctx.GetAdminCached(bg, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", cachedAdmin2.Nickname)
}

func TestGetAdminByUsernameCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create an admin
	admin := &model.Admin{
		Username: "cacheduser",
		Nickname: "Cached User",
		Email:    "cached@example.com",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password123")
	require.NoError(t, err)

	// Load by username
	cachedAdmin, err := ctx.GetAdminByUsernameCached(bg, "cacheduser")
	require.NoError(t, err)
	assert.Equal(t, "cacheduser", cachedAdmin.Username)
	assert.Equal(t, "Cached User", cachedAdmin.Nickname)

	// Test with whitespace username
	cachedAdmin2, err := ctx.GetAdminByUsernameCached(bg, "  cacheduser  ")
	require.NoError(t, err)
	assert.Equal(t, "cacheduser", cachedAdmin2.Username)

	// Test with empty username
	cachedAdmin3, err := ctx.GetAdminByUsernameCached(bg, "")
	require.NoError(t, err)
	assert.Nil(t, cachedAdmin3)
}

func TestGetAdminRolesCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create a role
	role := &model.Role{
		Name:        "testrole",
		Description: "Test Role",
		Category:    "test",
	}
	err := ctx.RoleModel.Create(bg, role)
	require.NoError(t, err)

	// Create an admin
	admin := &model.Admin{
		Username: "roleuser",
		Nickname: "Role User",
		Status:   1,
	}
	err = ctx.AdminModel.Create(bg, admin, "password123")
	require.NoError(t, err)

	// Assign role
	err = ctx.AdminModel.AssignRole(bg, admin.ID, role.ID)
	require.NoError(t, err)

	// Get roles cached
	roles, err := ctx.GetAdminRolesCached(bg, admin.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "testrole", roles[0].Name)
}

func TestInvalidateAdminCache(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create an admin
	admin := &model.Admin{
		Username: "invalidateuser",
		Nickname: "Invalidate User",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password123")
	require.NoError(t, err)

	// Load into cache
	_, err = ctx.GetAdminCached(bg, admin.ID)
	require.NoError(t, err)

	// Invalidate cache
	ctx.InvalidateAdminCache(bg, admin.ID, "invalidateuser")

	// Should not error
	ctx.InvalidateAdminCache(bg, 999, "")
	ctx.InvalidateAdminCache(bg, 0, "")
}

func TestInvalidateAdminRolesCache(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Should not panic
	ctx.InvalidateAdminRolesCache(bg, 1)
	ctx.InvalidateAdminRolesCache(bg, 0)
}

func TestCacheAdminAliases(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create an admin
	admin := &model.Admin{
		Username: "aliasuser",
		Nickname: "Alias User",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password123")
	require.NoError(t, err)

	// Call cacheAdminAliases
	ctx.cacheAdminAliases(bg, admin)

	// Test with nil admin
	ctx.cacheAdminAliases(bg, nil)

	// Test with nil context
	ctx.cacheAdminAliases(nil, admin)
}

func TestGetRoleCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create a role
	role := &model.Role{
		Name:        "cachedrole",
		Description: "Cached Role",
		Category:    "test",
	}
	err := ctx.RoleModel.Create(bg, role)
	require.NoError(t, err)

	// Get role cached
	cachedRole, err := ctx.GetRoleCached(bg, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "cachedrole", cachedRole.Name)
}

func TestGetRolePermissionIDsCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create permissions directly via DB
	perm1 := &model.Permission{
		ID:       "test.perm1",
		Name:     "Permission 1",
		Resource: "test",
		Action:   "perm1",
		Category: "test",
	}
	perm2 := &model.Permission{
		ID:       "test.perm2",
		Name:     "Permission 2",
		Resource: "test",
		Action:   "perm2",
		Category: "test",
	}
	err := ctx.DB.Create(perm1).Error
	require.NoError(t, err)
	err = ctx.DB.Create(perm2).Error
	require.NoError(t, err)

	// Create a role
	role := &model.Role{
		Name:        "permrole",
		Description: "Permission Role",
		Category:    "test",
	}
	err = ctx.RoleModel.Create(bg, role)
	require.NoError(t, err)

	// Assign permissions
	err = ctx.RoleModel.ReplacePermissions(bg, role.ID, []string{"test.perm1", "test.perm2"})
	require.NoError(t, err)

	// Get permission IDs cached
	permIDs, err := ctx.GetRolePermissionIDsCached(bg, role.ID)
	require.NoError(t, err)
	assert.Len(t, permIDs, 2)
	assert.Contains(t, permIDs, "test.perm1")
	assert.Contains(t, permIDs, "test.perm2")
}

func TestInvalidateRoleCache(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Should not panic
	ctx.InvalidateRoleCache(bg, 1)
	ctx.InvalidateRoleCache(bg, 0)
}

func TestGetPermissionCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create a permission directly via DB
	perm := &model.Permission{
		ID:       "cached.perm",
		Name:     "Cached Permission",
		Resource: "cached",
		Action:   "perm",
		Category: "cached",
	}
	err := ctx.DB.Create(perm).Error
	require.NoError(t, err)

	// Get permission cached
	cachedPerm, err := ctx.GetPermissionCached(bg, "cached.perm")
	require.NoError(t, err)
	assert.Equal(t, "cached.perm", cachedPerm.ID)

	// Test with whitespace
	cachedPerm2, err := ctx.GetPermissionCached(bg, "  CACHED.perm  ")
	require.NoError(t, err)
	assert.Equal(t, "cached.perm", cachedPerm2.ID)

	// Test with empty string
	cachedPerm3, err := ctx.GetPermissionCached(bg, "")
	require.NoError(t, err)
	assert.Nil(t, cachedPerm3)
}

func TestInvalidatePermissionCache(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Should not panic
	ctx.InvalidatePermissionCache(bg, "test.perm")
	ctx.InvalidatePermissionCache(bg, "")
	ctx.InvalidatePermissionCache(bg, "   ")
}

func TestGetGameCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create a game
	game := &model.Game{
		Name:      "testgame",
		AliasName: "Test Game",
		Status:    "dev",
		Enabled:   true,
		Color:     "#1677ff",
	}
	err := ctx.GameModel.Create(bg, game)
	require.NoError(t, err)

	// Get game cached
	cachedGame, err := ctx.GetGameCached(bg, game.ID)
	require.NoError(t, err)
	assert.Equal(t, "testgame", cachedGame.Name)
}

func TestListAllGamesCached(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Create games
	game1 := &model.Game{
		Name:      "game1",
		AliasName: "Game 1",
		Status:    "dev",
		Enabled:   true,
		Color:     "#1677ff",
	}
	game2 := &model.Game{
		Name:      "game2",
		AliasName: "Game 2",
		Status:    "prod",
		Enabled:   true,
		Color:     "#13c2c2",
	}
	err := ctx.GameModel.Create(bg, game1)
	require.NoError(t, err)
	err = ctx.GameModel.Create(bg, game2)
	require.NoError(t, err)

	// List all games cached
	games, err := ctx.ListAllGamesCached(bg)
	require.NoError(t, err)
	assert.Len(t, games, 2)
}

func TestInvalidateGameCache(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	bg := context.Background()

	// Should not panic
	ctx.InvalidateGameCache(bg, 1)
	ctx.InvalidateGameCache(bg, 0)
}

func TestDeleteCacheKey(t *testing.T) {
	t.Parallel()

	cacheStore := cache.NewNullCache()
	ctx := &ServiceContext{Cache: cacheStore}

	bg := context.Background()

	// Should not panic with various inputs
	ctx.deleteCacheKey(bg, "test:key")
	ctx.deleteCacheKey(bg, "")
	ctx.deleteCacheKey(nil, "test:key")
	ctx.deleteCacheKey(nil, "")

	// Test with nil cache
	ctx2 := &ServiceContext{Cache: nil}
	ctx2.deleteCacheKey(bg, "test:key")
}

// ============================================================================
// Tests for AdminManager
// ============================================================================

func TestAdminManager_ValidateUser(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"validuser","password":"pass123","roles":["admin"],"status":1}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)

	// Create empty roles and permissions files
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte("[]"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test valid user
	user, err := am.ValidateUser("validuser", "pass123")
	assert.NoError(t, err)
	assert.Equal(t, "validuser", user.Username)

	// Test invalid password
	_, err = am.ValidateUser("validuser", "wrongpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid password")

	// Test non-existent user
	_, err = am.ValidateUser("nonexistent", "pass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")

	// Test disabled user - manually create and disable
	err = am.CreateAdmin(&AdminUser{
		Username: "disableduser",
		Password: "pass123",
		Roles:    []string{"admin"},
	})
	require.NoError(t, err)

	// Now disable the user
	err = am.UpdateAdmin("disableduser", map[string]interface{}{"status": 0})
	require.NoError(t, err)

	_, err = am.ValidateUser("disableduser", "pass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is disabled")
}

func TestAdminManager_CreateAdmin(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	am := NewAdminManager(tmpDir)

	// Test creating admin
	newAdmin := &AdminUser{
		Username: "newadmin",
		Password: "newpass",
		Roles:    []string{"admin"},
		Nickname: "New Admin",
		Email:    "newadmin@test.com",
	}

	err := am.CreateAdmin(newAdmin)
	assert.NoError(t, err)
	assert.Equal(t, 1, newAdmin.Status)
	assert.NotEmpty(t, newAdmin.CreateAt)
	assert.NotEmpty(t, newAdmin.UpdateAt)

	// Test duplicate admin
	err = am.CreateAdmin(newAdmin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin already exists")
}

func TestAdminManager_UpdateAdmin(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"updateuser","password":"pass123","roles":["admin"],"nickname":"Old Name"}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte("[]"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test updating admin
	updates := map[string]interface{}{
		"nickname": "New Name",
		"email":    "newemail@test.com",
		"phone":    "1234567890",
		"roles":    []string{"admin", "editor"},
		"status":   1,
	}

	err = am.UpdateAdmin("updateuser", updates)
	assert.NoError(t, err)

	// Verify updates
	admin, err := am.GetAdmin("updateuser")
	assert.NoError(t, err)
	assert.Equal(t, "New Name", admin.Nickname)

	// Test non-existent admin
	err = am.UpdateAdmin("nonexistent", updates)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin not found")
}

func TestAdminManager_DeleteAdmin(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"deleteuser","password":"pass123","roles":["admin"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte("[]"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test deleting admin
	err = am.DeleteAdmin("deleteuser")
	assert.NoError(t, err)

	// Verify deletion
	_, err = am.GetAdmin("deleteuser")
	assert.Error(t, err)

	// Test deleting non-existent admin
	err = am.DeleteAdmin("nonexistent")
	assert.Error(t, err)
}

func TestAdminManager_ResetPassword(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"pwduser","password":"oldpass","roles":["admin"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte("[]"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test resetting password
	err = am.ResetPassword("pwduser", "newpass123")
	assert.NoError(t, err)

	// Verify new password works
	user, err := am.ValidateUser("pwduser", "newpass123")
	assert.NoError(t, err)
	assert.Equal(t, "pwduser", user.Username)

	// Test non-existent admin
	err = am.ResetPassword("nonexistent", "newpass")
	assert.Error(t, err)
}

func TestAdminManager_GetRole(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	rolesData := `[{"code":"test_role","name":"Test Role","description":"A test role","level":1,"permissions":["test.perm"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesData), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test getting existing role
	role, err := am.GetRole("test_role")
	assert.NoError(t, err)
	assert.Equal(t, "test_role", role.Code)
	assert.Equal(t, "Test Role", role.Name)

	// Test non-existent role
	_, err = am.GetRole("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role not found")
}

func TestAdminManager_CheckPermission(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"permuser","password":"pass123","roles":["editor"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)

	// Use a non-admin role name
	rolesData := `[{"code":"editor","name":"Editor","description":"Editor role","level":2,"permissions":["test.read","test.write"]}]`
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesData), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test specific permission that exists
	assert.True(t, am.CheckPermission("permuser", "test.read"))
	assert.True(t, am.CheckPermission("permuser", "test.write"))

	// Test non-existent permission
	assert.False(t, am.CheckPermission("permuser", "nonexistent.perm"))

	// Test non-existent user
	assert.False(t, am.CheckPermission("nonexistent", "test.read"))
}

func TestAdminManager_GetAdminPermissions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	adminData := `[{"username":"multiroleuser","password":"pass123","roles":["admin","editor"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)

	rolesData := `[{"code":"admin","name":"Admin","description":"Admin role","level":1,"permissions":["admin.read","admin.write"]},{"code":"editor","name":"Editor","description":"Editor role","level":2,"permissions":["content.read","content.write"]}]`
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesData), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte("[]"), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	require.NoError(t, err)

	// Test getting permissions for user with multiple roles
	perms := am.GetAdminPermissions("multiroleuser")
	assert.Len(t, perms, 4)
	assert.Contains(t, perms, "admin.read")
	assert.Contains(t, perms, "admin.write")
	assert.Contains(t, perms, "content.read")
	assert.Contains(t, perms, "content.write")

	// Test non-existent user
	perms = am.GetAdminPermissions("nonexistent")
	assert.Nil(t, perms)
}

func TestAdminManager_Initialize(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create all config files
	adminData := `[{"username":"inituser","password":"pass123","roles":["admin"]}]`
	err := os.WriteFile(filepath.Join(tmpDir, "admins.json"), []byte(adminData), 0o644)
	require.NoError(t, err)

	rolesData := `[{"code":"admin","name":"Admin","description":"Admin role","level":1,"permissions":["*"]}]`
	err = os.WriteFile(filepath.Join(tmpDir, "roles.json"), []byte(rolesData), 0o644)
	require.NoError(t, err)

	permsData := `[{"code":"*","name":"All","description":"All permissions","category":"admin","module":"*"}]`
	err = os.WriteFile(filepath.Join(tmpDir, "permissions.json"), []byte(permsData), 0o644)
	require.NoError(t, err)

	am := NewAdminManager(tmpDir)
	err = am.Initialize()
	assert.NoError(t, err)

	// Verify data loaded
	admins := am.ListAdmins()
	assert.Len(t, admins, 1)

	roles := am.ListRoles()
	assert.Len(t, roles, 1)

	perms := am.ListPermissions()
	assert.Len(t, perms, 1)
}

func TestAdminManager_Initialize_EmptyDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	am := NewAdminManager(tmpDir)

	// Should not error with empty directory
	err := am.Initialize()
	assert.NoError(t, err)

	// Lists should be empty
	admins := am.ListAdmins()
	assert.Empty(t, admins)

	roles := am.ListRoles()
	assert.Empty(t, roles)

	perms := am.ListPermissions()
	assert.Empty(t, perms)
}

// ============================================================================
// Tests for OpsStateStore
// ============================================================================

func TestNewOpsStateStore(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewOpsStateStore(tmpDir)

	assert.NotNil(t, store)
	assert.NotNil(t, store.state)
	assert.Equal(t, tmpDir, filepath.Dir(store.path))
}

func TestNewOpsStateStore_EmptyDir(t *testing.T) {
	t.Parallel()

	store := NewOpsStateStore("")

	assert.NotNil(t, store)
	assert.NotNil(t, store.state)
}

func TestOpsStateStore_Snapshot(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewOpsStateStore(tmpDir)

	snapshot := store.Snapshot()

	assert.NotNil(t, snapshot)
	assert.NotNil(t, snapshot.Config)
	assert.NotNil(t, snapshot.Maintenance)
}

func TestOpsStateStore_Update(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewOpsStateStore(tmpDir)

	// Update the state
	newState, err := store.Update(func(state *OpsState) {
		state.Config.AlertmanagerURL = "http://localhost:9093"
		state.Config.GrafanaExploreURL = "http://localhost:3000"
	})

	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:9093", newState.Config.AlertmanagerURL)
	assert.Equal(t, "http://localhost:3000", newState.Config.GrafanaExploreURL)

	// Verify persistence by loading again
	snapshot := store.Snapshot()
	assert.Equal(t, "http://localhost:9093", snapshot.Config.AlertmanagerURL)
}

func TestOpsStateStore_Update_Failure(t *testing.T) {
	t.Parallel()

	// Use invalid path
	store := &OpsStateStore{
		path:  "\x00\x00invalid",
		state: defaultOpsState(),
	}

	_, err := store.Update(func(state *OpsState) {
		state.Config.AlertmanagerURL = "test"
	})

	assert.Error(t, err)
}

func TestOpsStateStore_load(t *testing.T) {
	t.Parallel()

	t.Run("load with non-existent file creates default", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		store := &OpsStateStore{
			path:  filepath.Join(tmpDir, "new_state.json"),
			state: OpsState{},
		}

		err := store.load()
		assert.NoError(t, err)

		// File should be created
		_, err = os.Stat(store.path)
		assert.NoError(t, err)
	})

	t.Run("load existing file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "existing_state.json")

		// Create a state file
		store1 := NewOpsStateStore(tmpDir)
		store1.path = statePath
		_, err := store1.Update(func(state *OpsState) {
			state.Config.AlertmanagerURL = "http://test:9093"
		})
		require.NoError(t, err)

		// Load with new store
		store2 := &OpsStateStore{
			path:  statePath,
			state: OpsState{},
		}
		err = store2.load()
		assert.NoError(t, err)
		assert.Equal(t, "http://test:9093", store2.state.Config.AlertmanagerURL)
	})
}

func TestDefaultOpsState(t *testing.T) {
	t.Parallel()

	state := defaultOpsState()

	assert.NotNil(t, state)
	assert.NotNil(t, state.Config)
	assert.NotNil(t, state.MQ)
	assert.NotNil(t, state.Alerts)
	assert.NotNil(t, state.Audit)
	assert.NotEmpty(t, state.Audit.Entries)
}

func TestCloneOpsState(t *testing.T) {
	t.Parallel()

	original := OpsState{
		Config: OpsConfigState{
			AlertmanagerURL: "http://localhost:9093",
		},
	}

	cloned := cloneOpsState(original)

	assert.Equal(t, original.Config.AlertmanagerURL, cloned.Config.AlertmanagerURL)

	// Modify clone should not affect original
	cloned.Config.AlertmanagerURL = "http://changed:9093"
	assert.Equal(t, "http://localhost:9093", original.Config.AlertmanagerURL)
}

// ============================================================================
// Tests for DB Helpers
// ============================================================================

func TestEnsureSQLiteDir(t *testing.T) {
	t.Parallel()

	t.Run("memory database", func(t *testing.T) {
		t.Parallel()

		err := ensureSQLiteDir(":memory:")
		assert.NoError(t, err)
	})

	t.Run("empty DSN", func(t *testing.T) {
		t.Parallel()

		err := ensureSQLiteDir("")
		assert.NoError(t, err)
	})

	t.Run("valid file path creates directory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "subdir", "test.db")

		err := ensureSQLiteDir(dbPath)
		assert.NoError(t, err)

		// Verify directory created
		info, err := os.Stat(filepath.Dir(dbPath))
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("sqlite:// prefix", func(t *testing.T) {
		t.Parallel()

		// Skip on Windows where "sqlite:" contains invalid colon character for directory names
		// The ensureSQLiteDir function tries to create a directory from the path,
		// and Windows doesn't allow colons in directory names (except for drive letters)
		if filepath.Separator == '\\' {
			t.Skip("skipping on Windows due to invalid colon character in directory name")
		}

		tmpDir := t.TempDir()
		dbPath := "sqlite://" + tmpDir + "/test.db"

		err := ensureSQLiteDir(dbPath)
		assert.NoError(t, err)
	})

	t.Run("file: prefix", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dbPath := "file:" + filepath.Join(tmpDir, "test.db")

		err := ensureSQLiteDir(dbPath)
		assert.NoError(t, err)
	})

	t.Run("DSN with query parameters", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db") + "?cache=shared&mode=rwc"

		err := ensureSQLiteDir(dbPath)
		assert.NoError(t, err)
	})

	t.Run("current directory", func(t *testing.T) {
		t.Parallel()

		err := ensureSQLiteDir("test.db")
		assert.NoError(t, err)
	})
}

// ============================================================================
// Tests for Path Resolution Helpers
// ============================================================================

func TestToAbs(t *testing.T) {
	t.Parallel()

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		result := toAbs("")
		assert.Equal(t, "", result)
	})

	t.Run("absolute path", func(t *testing.T) {
		t.Parallel()

		if filepath.IsAbs("C:\\test") || filepath.IsAbs("/test") {
			absPath := "C:\\test"
			if filepath.Separator == '/' {
				absPath = "/test"
			}
			result := toAbs(absPath)
			assert.Equal(t, filepath.Clean(absPath), result)
		}
	})

	t.Run("relative path", func(t *testing.T) {
		t.Parallel()

		// Just verify it doesn't panic and returns something
		result := toAbs("relative/path")
		assert.NotEmpty(t, result)
	})
}

func TestResolveBootstrapAuthDir(t *testing.T) {
	t.Parallel()

	t.Run("with base dir set", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: tmpDir,
			},
		}

		result := resolveBootstrapAuthDir(cfg)
		assert.Equal(t, tmpDir, result)
	})

	t.Run("with empty base dir", func(t *testing.T) {
		t.Parallel()

		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: "",
			},
		}

		result := resolveBootstrapAuthDir(cfg)
		// Should return default directory
		assert.NotEmpty(t, result)
	})
}

func TestResolveBootstrapBaseDir(t *testing.T) {
	t.Parallel()

	t.Run("with base dir set", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: tmpDir,
			},
		}

		result := resolveBootstrapBaseDir(cfg)
		assert.Equal(t, tmpDir, result)
	})

	t.Run("with users config set", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		usersFile := filepath.Join(tmpDir, "users.json")
		cfg := config.Config{
			BootstrapData: config.BootstrapDataConfig{
				BaseDir: "",
			},
			Auth: config.AuthConfig{
				UsersConfig: usersFile,
			},
		}

		result := resolveBootstrapBaseDir(cfg)
		assert.Equal(t, tmpDir, result)
	})
}

func TestResolveJobRoutingDir(t *testing.T) {
	t.Parallel()

	t.Run("with routing dir set", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cfg := config.Config{
			AgentDispatch: config.AgentDispatchConfig{
				JobRoutingDir: tmpDir,
			},
		}

		result := resolveJobRoutingDir(cfg)
		assert.Equal(t, tmpDir, result)
	})

	t.Run("with empty routing dir", func(t *testing.T) {
		t.Parallel()

		cfg := config.Config{
			AgentDispatch: config.AgentDispatchConfig{
				JobRoutingDir: "",
			},
		}

		result := resolveJobRoutingDir(cfg)
		assert.Equal(t, "data", result)
	})
}

// ============================================================================
// Tests for Permission Helpers
// ============================================================================

func TestSplitPermissionCode(t *testing.T) {
	t.Parallel()

	t.Run("full permission code", func(t *testing.T) {
		t.Parallel()

		resource, action := splitPermissionCode("admin:write")
		assert.Equal(t, "admin", resource)
		assert.Equal(t, "write", action)
	})

	t.Run("permission code without action", func(t *testing.T) {
		t.Parallel()

		resource, action := splitPermissionCode("admin")
		assert.Equal(t, "admin", resource)
		assert.Equal(t, "*", action)
	})

	t.Run("empty permission code", func(t *testing.T) {
		t.Parallel()

		resource, action := splitPermissionCode("")
		assert.Equal(t, "", resource)
		assert.Equal(t, "", action) // Empty code returns empty action, not "*"
	})

	t.Run("permission with whitespace", func(t *testing.T) {
		t.Parallel()

		resource, action := splitPermissionCode("  admin  :  write  ")
		assert.Equal(t, "admin", resource)
		assert.Equal(t, "write", action)
	})

	t.Run("multiple colons", func(t *testing.T) {
		t.Parallel()

		resource, action := splitPermissionCode("admin:write:extra")
		assert.Equal(t, "admin", resource)
		assert.Equal(t, "write:extra", action)
	})
}

func TestDerivePermissionResourceAction(t *testing.T) {
	t.Parallel()

	t.Run("with module and full code", func(t *testing.T) {
		t.Parallel()

		resource, action := derivePermissionResourceAction("users:update", "user")
		assert.Equal(t, "user", resource)
		assert.Equal(t, "update", action)
	})

	t.Run("with empty module", func(t *testing.T) {
		t.Parallel()

		resource, action := derivePermissionResourceAction("users:update", "")
		assert.Equal(t, "users", resource)
		assert.Equal(t, "update", action)
	})

	t.Run("with empty code", func(t *testing.T) {
		t.Parallel()

		resource, action := derivePermissionResourceAction("", "user")
		assert.Equal(t, "user", resource)
		assert.Equal(t, "*", action)
	})

	t.Run("both empty", func(t *testing.T) {
		t.Parallel()

		resource, action := derivePermissionResourceAction("", "")
		assert.Equal(t, "global", resource)
		assert.Equal(t, "*", action)
	})
}

func TestMin(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, min(1, 5))
	assert.Equal(t, 2, min(5, 2))
	assert.Equal(t, 0, min(0, 0))
	assert.Equal(t, -5, min(-5, 5))
}

// ============================================================================
// Tests for Auth Middleware
// ============================================================================

func TestNewAuthMiddlewareImpl(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{}
	middleware := NewAuthMiddlewareImpl(ctx)

	assert.NotNil(t, middleware)
	assert.NotNil(t, middleware.allowPaths)
	assert.NotNil(t, middleware.allowPref)
	assert.NotNil(t, middleware.publicReadPrefixes)
	assert.NotNil(t, middleware.publicReadExactPaths)
}

func TestNewAuthMiddleware(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{}
	handler := NewAuthMiddleware(ctx)

	assert.NotNil(t, handler)
}

func TestAuthMiddleware_ShouldBypass(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{}
	middleware := NewAuthMiddlewareImpl(ctx)

	t.Run("public exact path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/auth/login", nil)
		assert.True(t, middleware.shouldBypass(req))
	})

	t.Run("health path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/monitoring/health", nil)
		assert.True(t, middleware.shouldBypass(req))
	})

	t.Run("public prefix path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/auth/login/extra", nil)
		assert.True(t, middleware.shouldBypass(req))
	})

	t.Run("public read prefix", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/configs", nil)
		assert.True(t, middleware.shouldBypass(req))
	})

	t.Run("public read exact path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/functions", nil)
		assert.True(t, middleware.shouldBypass(req))
	})

	t.Run("POST to public read prefix should not bypass", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("POST", "/api/v1/configs", nil)
		assert.False(t, middleware.shouldBypass(req))
	})

	t.Run("protected path", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/api/v1/admin", nil)
		assert.False(t, middleware.shouldBypass(req))
	})
}

func TestAuthMiddleware_Handle_MissingAuth(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx := &ServiceContext{}
	middleware := NewAuthMiddlewareImpl(ctx)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin", nil)

	middleware.Handle(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_Handle_InvalidHeaderFormat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx := &ServiceContext{}
	middleware := NewAuthMiddlewareImpl(ctx)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin", nil)
	c.Request.Header.Set("Authorization", "InvalidFormat token")

	middleware.Handle(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_Handle_BypassPublicPath(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	ctx := &ServiceContext{}
	middleware := NewAuthMiddlewareImpl(ctx)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/monitoring/health", nil)

	middleware.Handle(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// Tests for DBHealth
// ============================================================================

func TestNewDBHealth(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{}
	health := NewDBHealth(ctx)

	assert.NotNil(t, health)
	assert.Same(t, ctx, health.svcCtx)
}

func TestDBHealth_Check_Uninitialized(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{}
	health := NewDBHealth(ctx)

	err := health.Check(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库模型未初始化")
}

func TestDBHealth_Check_Success(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	health := NewDBHealth(ctx)

	// Create an admin first
	bg := context.Background()
	admin := &model.Admin{
		Username: "healthuser",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password")
	require.NoError(t, err)

	// Check should succeed since DB is connected
	err = health.Check(bg)
	assert.NoError(t, err)
}

func TestDBHealth_Ping(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	health := NewDBHealth(ctx)

	// Create an admin first
	bg := context.Background()
	admin := &model.Admin{
		Username: "pinguser",
		Status:   1,
	}
	err := ctx.AdminModel.Create(bg, admin, "password")
	require.NoError(t, err)

	err = health.Ping()
	assert.NoError(t, err)
}

// ============================================================================
// Tests for Seed Functions
// ============================================================================

func TestSeedBootstrapAdmins_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapAdmins(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapAdmins_NilModels(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{
		AdminManager: nil,
		AdminModel:   nil,
	}

	err := seedBootstrapAdmins(ctx)
	assert.NoError(t, err)
}

func TestSeedBootstrapRoles_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapRoles(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapPermissions_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapPermissions(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapGames_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapGames(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapTermDictionary_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapTermDictionary(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapWorkspaces_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapWorkspaces(nil)
	assert.NoError(t, err)
}

func TestEnsureWorkspaceSeeded(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)

	// Should not error
	err := ctx.EnsureWorkspaceSeeded()
	assert.NoError(t, err)
}

// ============================================================================
// Tests for AutoMigrate
// ============================================================================

func TestAutoMigrate(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)

	err := autoMigrate(db)
	assert.NoError(t, err)
}

// ============================================================================
// Tests for Game Seed Helpers
// ============================================================================

func TestSanitizeGameID(t *testing.T) {
	t.Parallel()

	t.Run("valid game ID", func(t *testing.T) {
		t.Parallel()

		result := sanitizeGameID("MyGame")
		assert.Equal(t, "mygame", result)
	})

	t.Run("with spaces", func(t *testing.T) {
		t.Parallel()

		result := sanitizeGameID("My Game")
		assert.Equal(t, "my_game", result)
	})

	t.Run("empty values", func(t *testing.T) {
		t.Parallel()

		result := sanitizeGameID("", "  ", "")
		assert.Equal(t, "", result)
	})

	t.Run("first non-empty value", func(t *testing.T) {
		t.Parallel()

		result := sanitizeGameID("", "  ", "GameOne", "GameTwo")
		assert.Equal(t, "gameone", result)
	})
}

func TestSanitizeAlias(t *testing.T) {
	t.Parallel()

	t.Run("valid alias", func(t *testing.T) {
		t.Parallel()

		result := sanitizeAlias("My Alias")
		assert.Equal(t, "My Alias", result)
	})

	t.Run("empty values", func(t *testing.T) {
		t.Parallel()

		result := sanitizeAlias("", "  ", "")
		assert.Equal(t, "", result)
	})

	t.Run("first non-empty value", func(t *testing.T) {
		t.Parallel()

		result := sanitizeAlias("", "  ", "Alias1", "Alias2")
		assert.Equal(t, "Alias1", result)
	})
}

func TestHumanizeGameID(t *testing.T) {
	t.Parallel()

	t.Run("underscore separated", func(t *testing.T) {
		t.Parallel()

		result := humanizeGameID("my_game_id")
		assert.Equal(t, "My Game Id", result)
	})

	t.Run("dash separated", func(t *testing.T) {
		t.Parallel()

		result := humanizeGameID("my-game-id")
		assert.Equal(t, "My Game Id", result)
	})

	t.Run("mixed separators", func(t *testing.T) {
		t.Parallel()

		result := humanizeGameID("my_game-id")
		assert.Equal(t, "My Game Id", result)
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()

		result := humanizeGameID("")
		assert.Equal(t, "", result)
	})

	t.Run("single word", func(t *testing.T) {
		t.Parallel()

		result := humanizeGameID("game")
		assert.Equal(t, "Game", result)
	})
}

func TestPickGameColor(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "#8c8c8c", pickGameColor(0))
	assert.Equal(t, "#1677ff", pickGameColor(1))
	assert.Equal(t, "#13c2c2", pickGameColor(2))

	// Test wrapping
	assert.Equal(t, "#8c8c8c", pickGameColor(len(fallbackGameColorCycle)))
}

func TestPickEnvColor(t *testing.T) {
	t.Parallel()

	t.Run("known env", func(t *testing.T) {
		t.Parallel()

		result := pickEnvColor("prod", nil)
		assert.Equal(t, "#13c2c2", result)
	})

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()

		result := pickEnvColor("PROD", nil)
		assert.Equal(t, "#13c2c2", result)
	})

	t.Run("unknown env", func(t *testing.T) {
		t.Parallel()

		result := pickEnvColor("unknown", nil)
		assert.Equal(t, defaultGameColor, result)
	})

	t.Run("from defaults map", func(t *testing.T) {
		t.Parallel()

		defaults := map[string]model.GameEnv{
			"custom": {Env: "custom", Color: "#ff0000"},
		}
		result := pickEnvColor("custom", defaults)
		assert.Equal(t, "#ff0000", result)
	})
}

func TestDefaultEnvColor(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "#13c2c2", defaultEnvColor("prod"))
	assert.Equal(t, "#fa8c16", defaultEnvColor("stage"))
	assert.Equal(t, "#722ed1", defaultEnvColor("test"))
	assert.Equal(t, "#1677ff", defaultEnvColor("dev"))
	assert.Equal(t, defaultGameColor, defaultEnvColor("unknown"))
}

func TestNormalizeColor(t *testing.T) {
	t.Parallel()

	t.Run("empty value returns fallback", func(t *testing.T) {
		t.Parallel()

		result := normalizeColor("", "#fallback")
		assert.Equal(t, "#fallback", result)
	})

	t.Run("hex color is lowercased", func(t *testing.T) {
		t.Parallel()

		result := normalizeColor("#ABCDEF", "#fallback")
		assert.Equal(t, "#abcdef", result)
	})

	t.Run("non-hex color returned as-is", func(t *testing.T) {
		t.Parallel()

		result := normalizeColor("red", "#fallback")
		assert.Equal(t, "red", result)
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		t.Parallel()

		result := normalizeColor("  #ABCDEF  ", "#fallback")
		assert.Equal(t, "#abcdef", result)
	})
}

func TestBoolPtr(t *testing.T) {
	t.Parallel()

	result := boolPtr(true)
	assert.NotNil(t, result)
	assert.True(t, *result)

	result = boolPtr(false)
	assert.NotNil(t, result)
	assert.False(t, *result)
}

func TestSanitizeEnvList(t *testing.T) {
	t.Parallel()

	t.Run("valid envs", func(t *testing.T) {
		t.Parallel()

		envs := []model.GameEnv{
			{Env: "prod", Description: "Production", Color: "#13c2c2"},
			{Env: "dev", Description: "Development", Color: "#1677ff"},
		}
		result := sanitizeEnvList(envs)

		assert.Len(t, result, 2)
		assert.Equal(t, "prod", result[0].Env)
		assert.Equal(t, "dev", result[1].Env)
	})

	t.Run("empty list returns defaults", func(t *testing.T) {
		t.Parallel()

		result := sanitizeEnvList([]model.GameEnv{})
		assert.NotEmpty(t, result)
	})

	t.Run("removes duplicates", func(t *testing.T) {
		t.Parallel()

		envs := []model.GameEnv{
			{Env: "prod", Description: "Production", Color: "#13c2c2"},
			{Env: "PROD", Description: "Production", Color: "#13c2c2"},
		}
		result := sanitizeEnvList(envs)

		assert.Len(t, result, 1)
	})

	t.Run("filters empty env names", func(t *testing.T) {
		t.Parallel()

		envs := []model.GameEnv{
			{Env: "", Description: "Empty", Color: "#13c2c2"},
			{Env: "prod", Description: "Production", Color: "#13c2c2"},
			{Env: "  ", Description: "Whitespace", Color: "#1677ff"},
		}
		result := sanitizeEnvList(envs)

		assert.Len(t, result, 1)
		assert.Equal(t, "prod", result[0].Env)
	})
}

func TestSanitizeGameEnvs(t *testing.T) {
	t.Parallel()

	t.Run("valid envs with defaults", func(t *testing.T) {
		t.Parallel()

		envs := []model.GameEnv{
			{Env: "custom", Description: "", Color: ""},
		}
		defaults := []model.GameEnv{
			{Env: "custom", Description: "Custom Env", Color: "#ff0000"},
		}
		result := sanitizeGameEnvs(envs, defaults)

		assert.Len(t, result, 1)
		assert.Equal(t, "Custom Env", result[0].Description)
		assert.Equal(t, "#ff0000", result[0].Color)
	})

	t.Run("empty list returns nil", func(t *testing.T) {
		t.Parallel()

		result := sanitizeGameEnvs([]model.GameEnv{}, nil)
		assert.Nil(t, result)
	})
}

func TestEnsureSeedEnvs(t *testing.T) {
	t.Parallel()

	t.Run("envs field takes precedence", func(t *testing.T) {
		t.Parallel()

		entry := bootstrapGameSeedEntry{
			GameID: "test",
			Envs: []model.GameEnv{
				{Env: "prod", Description: "Production", Color: "#13c2c2"},
			},
		}
		defaults := []model.GameEnv{}

		result := ensureSeedEnvs(entry, defaults)
		assert.Len(t, result, 1)
		assert.Equal(t, "prod", result[0].Env)
	})

	t.Run("env field used when envs empty", func(t *testing.T) {
		t.Parallel()

		entry := bootstrapGameSeedEntry{
			GameID: "test",
			Env:    "dev",
		}
		defaults := []model.GameEnv{}

		result := ensureSeedEnvs(entry, defaults)
		assert.Len(t, result, 1)
		assert.Equal(t, "dev", result[0].Env)
	})

	t.Run("returns nil when both empty", func(t *testing.T) {
		t.Parallel()

		entry := bootstrapGameSeedEntry{}
		defaults := []model.GameEnv{}

		result := ensureSeedEnvs(entry, defaults)
		assert.Nil(t, result)
	})
}

// ============================================================================
// Tests for Term Dictionary
// ============================================================================

func TestLoadTermDictionaryConfig_NilContext(t *testing.T) {
	t.Parallel()

	cfg := loadTermDictionaryConfig(nil)
	assert.NotEmpty(t, cfg.Items)
}

func TestDefaultTermDictionaryConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultTermDictionaryConfig()
	assert.NotEmpty(t, cfg.Items)

	// Check some expected entries
	domains := make(map[string]bool)
	for _, item := range cfg.Items {
		domains[item.Domain] = true
	}
	assert.True(t, domains["entity"])
	assert.True(t, domains["operation"])
}

// ============================================================================
// Tests for initObjectStore
// ============================================================================

func TestInitObjectStore_NoDriver(t *testing.T) {
	t.Parallel()

	cfg := config.StorageConfig{
		Driver: "",
	}

	store, err := initObjectStore(context.Background(), cfg)
	assert.NoError(t, err)
	assert.Nil(t, store)
}

func TestInitObjectStore_UnsupportedDriver(t *testing.T) {
	t.Parallel()

	cfg := config.StorageConfig{
		Driver: "unsupported",
	}

	store, err := initObjectStore(context.Background(), cfg)
	assert.Error(t, err)
	assert.Nil(t, store)
}

// ============================================================================
// Tests for Helpers
// ============================================================================

func TestBuildDefaultWorkspaceLayout(t *testing.T) {
	t.Parallel()

	functionIDs := []string{"examples.player.get", "examples.player.create", "examples.player.update"}

	layout := buildDefaultWorkspaceLayout("examples.player", functionIDs)

	assert.NotNil(t, layout)
	assert.Equal(t, "examples.player", layout["objectKey"])

	layoutData, ok := layout["layout"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "tabs", layoutData["type"])

	tabs, ok := layoutData["tabs"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, tabs, 3)
}

// ============================================================================
// Tests for seedBootstrapExtensionCatalog
// ============================================================================

func TestSeedBootstrapExtensionCatalog_NilContext(t *testing.T) {
	t.Parallel()

	err := seedBootstrapExtensionCatalog(nil)
	assert.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_NoDB(t *testing.T) {
	t.Parallel()

	ctx := &ServiceContext{DB: nil}
	err := seedBootstrapExtensionCatalog(ctx)
	assert.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_MissingFile(t *testing.T) {
	t.Parallel()

	ctx := setupTestServiceContext(t)
	// Point to a directory without extensions catalog
	ctx.Config.BootstrapData.BaseDir = t.TempDir()

	err := seedBootstrapExtensionCatalog(ctx)
	// Should not error if file doesn't exist
	assert.NoError(t, err)
}

// ============================================================================
// Tests for openDatabase errors
// ============================================================================

func TestOpenDatabase_PostgresNoDSN(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "postgres",
			DataSource: "",
		},
	}

	_, err := openDatabase(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "postgres DSN is required")
}

func TestOpenDatabase_MySQLNoDSN(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "mysql",
			DataSource: "",
		},
	}

	_, err := openDatabase(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mysql DSN is required")
}

func TestOpenDatabase_SQLServerNoDSN(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "sqlserver",
			DataSource: "",
		},
	}

	_, err := openDatabase(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sqlserver DSN is required")
}

func TestOpenDatabase_UnsupportedDriver(t *testing.T) {
	t.Parallel()

	// Clear env vars that might override config
	os.Unsetenv("DB_DRIVER")
	os.Unsetenv("DATABASE_URL")

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "mongodb",
			DataSource: "",
		},
	}

	_, err := openDatabase(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver")
}

func TestOpenDatabase_SQLite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "sqlite",
			DataSource: dbPath,
		},
	}

	db, err := openDatabase(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Verify connection works
	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Ping())

	// Close the database to allow cleanup
	sqlDB.Close()
}

func TestOpenDatabase_Auto(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "auto",
			DataSource: ":memory:",
		},
	}

	db, err := openDatabase(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, db)
}

func TestOpenDatabase_EnvOverride(t *testing.T) {
	// Note: Can't use t.Parallel() with t.Setenv()

	// Test DB_DRIVER env override
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DATABASE_URL", ":memory:")

	cfg := config.Config{
		Database: config.DatabaseConfig{
			Driver:     "postgres",
			DataSource: "postgres://localhost",
		},
	}

	db, err := openDatabase(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, db)
}
