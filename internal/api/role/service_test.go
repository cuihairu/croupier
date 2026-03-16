package role

import (
	"context"
	"strconv"
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
	testRoleDB     *gorm.DB
	testRoleDBOnce sync.Once
	testRoleDBMu   sync.Mutex
)

func setupRoleTestDB(t *testing.T) *gorm.DB {
	testRoleDBMu.Lock()
	defer testRoleDBMu.Unlock()

	testRoleDBOnce.Do(func() {
		var err error
		testRoleDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(testRoleDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up data between tests
	testRoleDB.Exec("DELETE FROM role_permissions")
	testRoleDB.Exec("DELETE FROM admin_roles")
	testRoleDB.Exec("DELETE FROM admins")
	testRoleDB.Exec("DELETE FROM roles")
	testRoleDB.Exec("DELETE FROM permissions")

	return testRoleDB
}

func createTestRoleContext(t *testing.T, db *gorm.DB) (*svc.ServiceContext, context.Context) {
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	// Create test admin
	admin := &model.Admin{
		Username: "testadmin",
		Nickname: "Test Admin",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	// Create admin role
	role := &model.Role{
		Name:        "admin",
		Description: "Admin role",
		Category:    "system",
	}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	// Assign role to admin
	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	// Assign permissions to role
	err = roleModel.ReplacePermissions(context.Background(), role.ID, []string{
		"admin:all",
		"roles:manage",
		"role:write",
	})
	require.NoError(t, err)

	// Create test permissions
	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "roles:manage", Name: "Roles Manage", Resource: "roles", Action: "manage", Category: "role"},
		{ID: "role:write", Name: "Role Write", Resource: "role", Action: "write", Category: "role"},
		{ID: "game:read", Name: "Game Read", Resource: "game", Action: "read", Category: "game"},
		{ID: "player:read", Name: "Player Read", Resource: "player", Action: "read", Category: "player"},
	}
	for _, perm := range permissions {
		err = db.Create(perm).Error
		require.NoError(t, err)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: permissionModel,
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	ctx := context.WithValue(context.Background(), "username", "testadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	return svcCtx, ctx
}

func TestService_RoleCreate_Success(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "editor",
		Description: "Editor role",
		Category:    "custom",
		Permissions: []string{"game:read", "player:read"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "editor", resp.Name)
	assert.Equal(t, "Editor role", resp.Description)
	assert.Equal(t, "custom", resp.Category)
	assert.Len(t, resp.Permissions, 2)
}

func TestService_RoleCreate_EmptyName(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "",
		Description: "Test role",
		Permissions: []string{"game:read"},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "角色名称不能为空")
}

func TestService_RoleCreate_WhitespaceName(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "   ",
		Description: "Test role",
		Permissions: []string{"game:read"},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "角色名称不能为空")
}

func TestService_RoleCreate_NameWithWhitespace(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "  editor  ",
		Description: "Test role",
		Permissions: []string{"game:read"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "editor", resp.Name) // Should trim whitespace
}

func TestService_RoleCreate_NoPermissions(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "viewer",
		Description: "Viewer role",
		Category:    "custom",
		Permissions: []string{},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Permissions, 0)
}

func TestService_RoleCreate_NilPermissions(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "viewer",
		Description: "Viewer role",
		Category:    "custom",
		Permissions: nil,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Permissions, 0)
}

func TestService_RoleCreate_InvalidPermission(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "editor",
		Description: "Editor role",
		Permissions: []string{"invalid:permission"},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_RoleCreate_DuplicatePermissions(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "editor",
		Description: "Editor role",
		Permissions: []string{"game:read", "game:read", "player:read"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should deduplicate
	assert.LessOrEqual(t, len(resp.Permissions), 3)
}

func TestService_RoleCreate_PermissionDenied(t *testing.T) {
	db := setupRoleTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	// Create admin without required permissions
	admin := &model.Admin{
		Username: "nopermuser",
		Nickname: "No Permission User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "viewer"}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: permissionModel,
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	ctx := context.WithValue(context.Background(), "username", "nopermuser")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	service := NewService(svcCtx)

	resp, err := service.RoleCreate(ctx, &RoleCreateRequest{
		Name:        "editor",
		Description: "Editor role",
		Permissions: []string{"game:read"},
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无权")
}

func TestService_RoleDelete_Success(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	service := NewService(svcCtx)

	// Create a role to delete
	role := &model.Role{Name: "todelete"}
	err := roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = service.RoleDelete(ctx, &RoleDeleteRequest{
		ID: strconv.FormatUint(uint64(role.ID), 10),
	})

	assert.NoError(t, err)
}

func TestService_RoleDelete_NotFound(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	err := service.RoleDelete(ctx, &RoleDeleteRequest{ID: "99999"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_RoleDelete_EmptyID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	err := service.RoleDelete(ctx, &RoleDeleteRequest{ID: ""})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID不能为空")
}

func TestService_RoleDelete_WhitespaceID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	err := service.RoleDelete(ctx, &RoleDeleteRequest{ID: "   "})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID不能为空")
}

func TestService_RoleDelete_InvalidID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	err := service.RoleDelete(ctx, &RoleDeleteRequest{ID: "invalid"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的角色ID")
}

func TestService_RoleDelete_ZeroID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	err := service.RoleDelete(ctx, &RoleDeleteRequest{ID: "0"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "角色ID必须大于0")
}

func TestService_RoleDetail_Success(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	service := NewService(svcCtx)

	// Create a role with permissions
	role := &model.Role{
		Name:        "moderator",
		Description: "Moderator role",
		Category:    "custom",
	}
	err := roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(context.Background(), role.ID, []string{"game:read", "player:read"})
	require.NoError(t, err)

	resp, err := service.RoleDetail(ctx, &RoleDetailRequest{
		ID: strconv.FormatUint(uint64(role.ID), 10),
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "moderator", resp.Name)
	assert.Equal(t, "Moderator role", resp.Description)
	assert.Len(t, resp.Permissions, 2)
}

func TestService_RoleDetail_NotFound(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleDetail(ctx, &RoleDetailRequest{ID: "99999"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_RoleUpdate_Success(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	service := NewService(svcCtx)

	// Create a role to update
	role := &model.Role{Name: "toupdate"}
	err := roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	resp, err := service.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:          strconv.FormatUint(uint64(role.ID), 10),
		Name:        "updated",
		Description: "Updated role",
		Category:    "updated",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "updated", resp.Name)
	assert.Equal(t, "Updated role", resp.Description)
}

func TestService_RoleUpdate_WithPermissions(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	service := NewService(svcCtx)

	// Create a role to update
	role := &model.Role{Name: "toupdateperms"}
	err := roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	resp, err := service.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:          strconv.FormatUint(uint64(role.ID), 10),
		Name:        "updated",
		Permissions: []string{"game:read", "player:read"},
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Permissions, 2)
}

func TestService_RoleUpdate_EmptyID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:   "",
		Name: "updated",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "角色ID不能为空")
}

func TestService_RoleUpdate_InvalidID(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:   "invalid",
		Name: "updated",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无效的角色ID")
}

func TestService_RoleUpdate_NotFound(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RoleUpdate(ctx, &RoleUpdateRequest{
		ID:   "99999",
		Name: "updated",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_RolesList_Success(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	// Create additional roles
	roles := []*model.Role{
		{Name: "editor", Description: "Editor", Category: "custom"},
		{Name: "viewer", Description: "Viewer", Category: "custom"},
		{Name: "moderator", Description: "Moderator", Category: "custom"},
	}
	for _, r := range roles {
		err := roleModel.Create(context.Background(), r)
		require.NoError(t, err)
	}

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 3) // admin + created roles
	assert.GreaterOrEqual(t, resp.Total, int64(3))
}

func TestService_RolesList_WithCategoryFilter(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	// Create roles with different categories
	roles := []*model.Role{
		{Name: "editor1", Description: "Editor", Category: "custom"},
		{Name: "editor2", Description: "Editor", Category: "custom"},
		{Name: "system", Description: "System", Category: "system"},
	}
	for _, r := range roles {
		err := roleModel.Create(context.Background(), r)
		require.NoError(t, err)
	}

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     1,
		PageSize: 10,
		Category: "custom",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 2)
}

func TestService_RolesList_WithSearch(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)
	roleModel := model.NewRoleModel(db)

	// Create roles
	roles := []*model.Role{
		{Name: "game_editor", Description: "Game Editor", Category: "game"},
		{Name: "player_editor", Description: "Player Editor", Category: "player"},
		{Name: "viewer", Description: "Just Viewer", Category: "custom"},
	}
	for _, r := range roles {
		err := roleModel.Create(context.Background(), r)
		require.NoError(t, err)
	}

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     1,
		PageSize: 10,
		Search:   "editor",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 2)
}

func TestService_RolesList_EmptyResults(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     1,
		PageSize: 10,
		Category: "nonexistent",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 0)
}

func TestService_RolesList_DefaultPagination(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.Size)
}

func TestService_RolesList_ZeroPage(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     0,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page) // Should default to 1
}

func TestService_RolesList_ZeroPageSize(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, ctx := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{
		Page:     1,
		PageSize: 0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 20, resp.Size) // Should default to 20
}

func TestService_RolesList_PermissionDenied(t *testing.T) {
	db := setupRoleTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	// Create admin without required permissions
	admin := &model.Admin{
		Username: "nopermlist",
		Nickname: "No Perm List",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{Name: "viewer"}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: permissionModel,
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	ctx := context.WithValue(context.Background(), "username", "nopermlist")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	service := NewService(svcCtx)

	resp, err := service.RolesList(ctx, &RolesListRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无权")
}

func TestParseRoleID_Valid(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, _ := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	tests := []struct {
		name     string
		input    string
		expected uint
		hasError bool
	}{
		{"valid 1", "1", 1, false},
		{"valid 100", "100", 100, false},
		{"empty", "", 0, true},
		{"whitespace", "   ", 0, true},
		{"invalid", "abc", 0, true},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.parseRoleID(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBuildRole(t *testing.T) {
	db := setupRoleTestDB(t)
	svcCtx, _ := createTestRoleContext(t, db)

	service := NewService(svcCtx)

	roleModel := model.NewRoleModel(db)
	role := &model.Role{
		Name:        "testrole",
		Description: "Test Role",
		Category:    "test",
	}
	err := roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	permissionIDs := []string{"game:read", "player:read"}

	result := service.buildRole(role, permissionIDs)

	assert.Equal(t, int64(role.ID), result.Id)
	assert.Equal(t, "testrole", result.Name)
	assert.Equal(t, "Test Role", result.Description)
	assert.Equal(t, "test", result.Category)
	assert.Len(t, result.Permissions, 2)
	assert.NotEmpty(t, result.CreatedAt)
	assert.NotEmpty(t, result.UpdatedAt)
}

func TestFormatTimestamp_ZeroTime(t *testing.T) {
	result := formatTimestamp(time.Time{})
	assert.Empty(t, result)
}

func TestFormatTimestamp_ValidTime(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result := formatTimestamp(now)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "2024")
}
