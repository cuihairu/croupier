package permission

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
	testPermissionDB     *gorm.DB
	testPermissionDBOnce sync.Once
	testPermissionDBMu   sync.Mutex
)

func setupPermissionTestDB(t *testing.T) *gorm.DB {
	testPermissionDBMu.Lock()
	defer testPermissionDBMu.Unlock()

	testPermissionDBOnce.Do(func() {
		var err error
		testPermissionDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(testPermissionDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up data between tests
	testPermissionDB.Exec("DELETE FROM role_permissions")
	testPermissionDB.Exec("DELETE FROM permissions")
	testPermissionDB.Exec("DELETE FROM admin_roles")
	testPermissionDB.Exec("DELETE FROM admins")
	testPermissionDB.Exec("DELETE FROM roles")

	return testPermissionDB
}

func createTestPermissionContext(t *testing.T, db *gorm.DB) (*svc.ServiceContext, context.Context) {
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
		"roles:read",
		"role:read",
		"roles:manage",
		"role:write",
	})
	require.NoError(t, err)

	// Create test permissions
	permissions := []*model.Permission{
		{ID: "admin:all", Name: "Admin All", Resource: "admin", Action: "all", Category: "admin"},
		{ID: "roles:read", Name: "Roles Read", Resource: "roles", Action: "read", Category: "role"},
		{ID: "role:read", Name: "Role Read", Resource: "role", Action: "read", Category: "role"},
		{ID: "roles:manage", Name: "Roles Manage", Resource: "roles", Action: "manage", Category: "role"},
		{ID: "role:write", Name: "Role Write", Resource: "role", Action: "write", Category: "role"},
		{ID: "game:read", Name: "Game Read", Resource: "game", Action: "read", Category: "game"},
		{ID: "player:read", Name: "Player Read", Resource: "player", Action: "read", Category: "player"},
		{ID: "config:read", Name: "Config Read", Resource: "config", Action: "read", Category: "config"},
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

func TestService_List_Success(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, len(resp.Items), 5)
	assert.GreaterOrEqual(t, resp.Total, int64(5))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.Size)
}

func TestService_List_WithCategoryFilter(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
		Category: "game",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "game:read", resp.Items[0].Id)
}

func TestService_List_WithResourceFilter(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
		Resource: "role",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// permissions with resource=role are: roles:read, role:read, roles:manage, role:write
	assert.GreaterOrEqual(t, len(resp.Items), 2)
}

func TestService_List_WithBothFilters(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
		Category: "role",
		Resource: "role",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Should get permissions with both category=role and resource=role
}

func TestService_List_EmptyResults(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 10,
		Category: "nonexistent",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 0)
	assert.Equal(t, int64(0), resp.Total)
}

func TestService_List_DefaultPagination(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.Size)
}

func TestService_List_ZeroPage(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     0,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Page) // Should default to 1
}

func TestService_List_ZeroPageSize(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 20, resp.Size) // Should default to 20
}

func TestService_List_PermissionDenied(t *testing.T) {
	db := setupPermissionTestDB(t)
	// Create admin without required permissions
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	admin := &model.Admin{
		Username: "nopermuser",
		Nickname: "No Permission User",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	role := &model.Role{
		Name:        "viewer",
		Description: "Viewer role",
	}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	// Create some permissions
	for _, perm := range []*model.Permission{
		{ID: "test:read", Name: "Test Read", Resource: "test", Action: "read", Category: "test"},
	} {
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

	ctx := context.WithValue(context.Background(), "username", "nopermuser")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无权")
}

func TestService_Detail_Success(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{
		ID: "game:read",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "game:read", resp.Id)
	assert.Equal(t, "Game Read", resp.Name)
	assert.Equal(t, "game", resp.Resource)
	assert.Equal(t, "read", resp.Action)
	assert.Equal(t, "game", resp.Category)
}

func TestService_Detail_NotFound(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{
		ID: "notfound:xyz",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_Detail_EmptyID(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{
		ID: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "权限ID不能为空")
}

func TestService_Detail_WhitespaceID(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{
		ID: "   ",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "权限ID不能为空")
}

func TestService_Detail_IDWithWhitespace(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{
		ID: "  game:read  ",
	})

	assert.NoError(t, err) // Should trim whitespace
	assert.NotNil(t, resp)
	assert.Equal(t, "game:read", resp.Id)
}

func TestService_Detail_PermissionDenied(t *testing.T) {
	db := setupPermissionTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	admin := &model.Admin{
		Username: "nopermdetail",
		Nickname: "No Perm Detail",
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

	ctx := context.WithValue(context.Background(), "username", "nopermdetail")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{ID: "game:read"})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "无权")
}

func TestService_Detail_AdminRoleWildcard(t *testing.T) {
	db := setupPermissionTestDB(t)
	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)
	permissionModel := model.NewPermissionModel(db)

	admin := &model.Admin{
		Username: "superadmin",
		Nickname: "Super Admin",
		Status:   1,
	}
	err := adminModel.Create(context.Background(), admin, "password123")
	require.NoError(t, err)

	// Create admin role with admin:all permission
	role := &model.Role{Name: "admin"}
	err = roleModel.Create(context.Background(), role)
	require.NoError(t, err)

	err = adminModel.AssignRole(context.Background(), admin.ID, role.ID)
	require.NoError(t, err)

	err = roleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"})
	require.NoError(t, err)

	perm := &model.Permission{ID: "test:custom", Name: "Test Custom", Resource: "test", Action: "custom", Category: "test"}
	err = db.Create(perm).Error
	require.NoError(t, err)

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: permissionModel,
		Cache:           cache.NewNullCache(),
		CacheHelper:     cache.NewCacheHelper(cache.NewNullCache()),
	}

	ctx := context.WithValue(context.Background(), "username", "superadmin")
	ctx = context.WithValue(ctx, "adminID", admin.ID)

	service := NewService(svcCtx)

	resp, err := service.Detail(ctx, &PermissionDetailRequest{ID: "test:custom"})

	assert.NoError(t, err) // Admin role with admin:all should have access
	assert.NotNil(t, resp)
	assert.Equal(t, "test:custom", resp.Id)
}

func TestBuildPermission(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	perm := model.Permission{
		ID:          "test:read",
		Name:        "Test Read",
		Description: "Test permission",
		Resource:    "test",
		Action:      "read",
		Category:    "test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result := buildPermission(perm)

	assert.Equal(t, "test:read", result.Id)
	assert.Equal(t, "Test Read", result.Name)
	assert.Equal(t, "Test permission", result.Description)
	assert.Equal(t, "test", result.Resource)
	assert.Equal(t, "read", result.Action)
	assert.Equal(t, "test", result.Category)
	assert.NotEmpty(t, result.CreatedAt)
	assert.NotEmpty(t, result.UpdatedAt)
}

func TestFormatTime_ZeroTime(t *testing.T) {
	var zero time.Time
	result := formatTime(zero)
	assert.Empty(t, result) // Zero time should return empty string
}

func TestFormatTime_ValidTime(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result := formatTime(now)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "2024")
}

func TestService_List_WithPageSize2(t *testing.T) {
	db := setupPermissionTestDB(t)
	svcCtx, ctx := createTestPermissionContext(t, db)

	service := NewService(svcCtx)

	resp, err := service.List(ctx, &PermissionsListRequest{
		Page:     1,
		PageSize: 2,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, 2, resp.Size)
}
