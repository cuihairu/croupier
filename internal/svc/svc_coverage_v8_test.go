package svc

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- admin_manager.go: Initialize error paths ---

func TestInitialize_BadAdminsJSONV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`{invalid`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	// Bad JSON is logged but Initialize continues successfully
	require.NoError(t, err)
}

func TestInitialize_BadRolesJSONV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`{bad`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.Error(t, err)
}

func TestInitialize_BadPermissionsJSONV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[bad`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.Error(t, err)
}

func TestInitialize_NoFilesV8(t *testing.T) {
	dir := t.TempDir()
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
}

func TestInitialize_UsersFallbackV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "users.json"), []byte(`[{"username":"u1","password":"p1"}]`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
	admin, err := am.GetAdmin("u1")
	require.NoError(t, err)
	assert.Equal(t, "u1", admin.Username)
}

func TestInitialize_RolesNotExistV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
}

func TestInitialize_PermissionsNotExistV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[]`), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
}

func TestLoadDefaultAdmins_ReadErrorV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "admins.json"), 0o755))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
}

func TestLoadDefaultAdmins_DuplicatedAdminV8(t *testing.T) {
	dir := t.TempDir()
	admins := `[{"username":"dup","password":"p1","status":1},{"username":"dup","password":"p2","status":1}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(admins), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
	list := am.ListAdmins()
	assert.Len(t, list, 1)
}

func TestLoadDefaultAdmins_AlreadyLoadedV8(t *testing.T) {
	dir := t.TempDir()
	admins := `[{"username":"existing","password":"p1"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(admins), 0o644))
	am := NewAdminManager(dir)
	err := am.CreateAdmin(&AdminUser{Username: "existing", Password: "p1"})
	require.NoError(t, err)
	err = am.Initialize()
	require.NoError(t, err)
}

func TestLoadDefaultAdmins_ZeroStatusV8(t *testing.T) {
	dir := t.TempDir()
	admins := `[{"username":"zero_status","password":"p1","status":0}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(admins), 0o644))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.NoError(t, err)
	admin, err := am.GetAdmin("zero_status")
	require.NoError(t, err)
	assert.Equal(t, 1, admin.Status)
}

func TestLoadDefaultRoles_ReadErrorV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "roles.json"), 0o755))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.Error(t, err)
}

func TestLoadDefaultPermissions_ReadErrorV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "permissions.json"), 0o755))
	am := NewAdminManager(dir)
	err := am.Initialize()
	require.Error(t, err)
}

func TestValidateUser_DisabledV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "dis", Password: "pass"})
	require.NoError(t, err)
	err = am.UpdateAdmin("dis", map[string]interface{}{"status": 0})
	require.NoError(t, err)
	_, err = am.ValidateUser("dis", "pass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestValidateUser_PlaintextPasswordV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "plain", Password: "secret"})
	require.NoError(t, err)
	admin, err := am.ValidateUser("plain", "secret")
	require.NoError(t, err)
	assert.Equal(t, "plain", admin.Username)
}

func TestValidateUser_WrongPlaintextV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "wrong", Password: "secret"})
	require.NoError(t, err)
	_, err = am.ValidateUser("wrong", "wrongpass")
	require.Error(t, err)
}

func TestResetPassword_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.ResetPassword("nobody", "newpass")
	require.Error(t, err)
}

func TestResetPassword_PlaintextV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "rp", Password: "old"})
	require.NoError(t, err)
	err = am.ResetPassword("rp", "newpass")
	require.NoError(t, err)
	admin, err := am.GetAdmin("rp")
	require.NoError(t, err)
	assert.True(t, admin.IsHashedPassword())
}

func TestResetPassword_AlreadyHashedV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "rp2", Password: "old"})
	require.NoError(t, err)
	err = am.ResetPassword("rp2", "$2a$10$dummyhashvalue123456789012345678901234567890")
	require.NoError(t, err)
}

func TestGetAdmin_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	_, err := am.GetAdmin("nobody")
	require.Error(t, err)
}

func TestDeleteAdmin_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.DeleteAdmin("nobody")
	require.Error(t, err)
}

func TestUpdateAdmin_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.UpdateAdmin("nobody", map[string]interface{}{"nickname": "x"})
	require.Error(t, err)
}

func TestCheckPermission_AdminRoleV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "adm", Password: "p", Roles: []string{"admin"}})
	require.NoError(t, err)
	assert.True(t, am.CheckPermission("adm", "anything"))
}

func TestCheckPermission_SuperAdminRoleV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "sa", Password: "p", Roles: []string{"super_admin"}})
	require.NoError(t, err)
	assert.True(t, am.CheckPermission("sa", "anything"))
}

func TestCheckPermission_WildcardPermV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"r1","name":"R1","permissions":["*"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[]`), 0o644))
	am := NewAdminManager(dir)
	err := am.CreateAdmin(&AdminUser{Username: "wild", Password: "p", Roles: []string{"r1"}})
	require.NoError(t, err)
	err = am.Initialize()
	require.NoError(t, err)
	assert.True(t, am.CheckPermission("wild", "game:read"))
}

func TestCheckPermission_AdminAllPermV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"r2","name":"R2","permissions":["admin:all"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[]`), 0o644))
	am := NewAdminManager(dir)
	err := am.CreateAdmin(&AdminUser{Username: "aa", Password: "p", Roles: []string{"r2"}})
	require.NoError(t, err)
	err = am.Initialize()
	require.NoError(t, err)
	assert.True(t, am.CheckPermission("aa", "game:write"))
}

func TestCheckPermission_ExactMatchV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"r3","name":"R3","permissions":["game:read"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[]`), 0o644))
	am := NewAdminManager(dir)
	err := am.CreateAdmin(&AdminUser{Username: "exact", Password: "p", Roles: []string{"r3"}})
	require.NoError(t, err)
	err = am.Initialize()
	require.NoError(t, err)
	assert.True(t, am.CheckPermission("exact", "game:read"))
	assert.False(t, am.CheckPermission("exact", "game:write"))
}

func TestCheckPermission_NonexistentUserV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	assert.False(t, am.CheckPermission("ghost", "perm"))
}

func TestCheckPermission_NoMatchingPermV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "noperm", Password: "p", Roles: []string{"viewer"}})
	require.NoError(t, err)
	assert.False(t, am.CheckPermission("noperm", "game:write"))
}

func TestGetAdminPermissions_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	perms := am.GetAdminPermissions("ghost")
	assert.Nil(t, perms)
}

func TestListRolesV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"admin","name":"Admin","description":"admin role"}]`), 0o644))
	am := NewAdminManager(dir)
	_ = am.Initialize()
	roles := am.ListRoles()
	assert.Len(t, roles, 1)
}

func TestListPermissionsV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[{"code":"p1","name":"P1","description":"perm1","category":"cat","module":"mod"}]`), 0o644))
	am := NewAdminManager(dir)
	_ = am.Initialize()
	perms := am.ListPermissions()
	assert.Len(t, perms, 1)
}

func TestGetRole_NotFoundV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	_, err := am.GetRole("ghost")
	require.Error(t, err)
}

func TestGetRole_FoundV8(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"admin","name":"Admin"}]`), 0o644))
	am := NewAdminManager(dir)
	_ = am.Initialize()
	role, err := am.GetRole("admin")
	require.NoError(t, err)
	assert.Equal(t, "Admin", role.Name)
}

func TestCreateAdmin_DuplicateV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	err := am.CreateAdmin(&AdminUser{Username: "dup", Password: "p"})
	require.NoError(t, err)
	err = am.CreateAdmin(&AdminUser{Username: "dup", Password: "p2"})
	require.Error(t, err)
}

func TestIsHashedPasswordV8(t *testing.T) {
	u := &AdminUser{Password: "$2a$10$dummy"}
	assert.True(t, u.IsHashedPassword())
	u2 := &AdminUser{Password: "$2b$10$dummy"}
	assert.True(t, u2.IsHashedPassword())
	u3 := &AdminUser{Password: "plaintext"}
	assert.False(t, u3.IsHashedPassword())
}

// --- ops_state_store.go: edge cases ---

func TestOpsStateStore_EmptyBaseDirV8(t *testing.T) {
	store := NewOpsStateStore("")
	require.NotNil(t, store)
}

func TestOpsStateStore_CloneErrorV8(t *testing.T) {
	st := defaultOpsState()
	cp := cloneOpsState(st)
	assert.Equal(t, st.Config.AlertmanagerURL, cp.Config.AlertmanagerURL)
}

func TestOpsStateStore_LoadNotExistV8(t *testing.T) {
	dir := t.TempDir()
	store := NewOpsStateStore(dir)
	require.NotNil(t, store)
	snap := store.Snapshot()
	assert.False(t, snap.Config.UpdatedAt.IsZero())
}

// --- service_context.go: seedBootstrapAdmins edge cases ---

func TestSeedBootstrapAdmins_NilCtxV8(t *testing.T) {
	err := seedBootstrapAdmins(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapAdmins_EmptyAdminsV8(t *testing.T) {
	dir := t.TempDir()
	svcCtx := &ServiceContext{
		AdminManager: NewAdminManager(dir),
		AdminModel:   model.NewAdminModel(newSvcTestContext(t).DB),
	}
	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapAdmins_NoAdminModelV8(t *testing.T) {
	dir := t.TempDir()
	svcCtx := &ServiceContext{
		AdminManager: NewAdminManager(dir),
	}
	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapAdmins_PlaintextPasswordV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"admin1","password":"plaintext123","roles":["admin"]}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)

	var dbAdmin model.Admin
	err = svcCtx.DB.Where("username = ?", "admin1").First(&dbAdmin).Error
	require.NoError(t, err)
	assert.Equal(t, "admin1", dbAdmin.Username)
}

func TestSeedBootstrapAdmins_ExistingAdminUpdateStatusV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"sync_user_v8","password":"pass","status":0}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())
	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
	var dbAdmin model.Admin
	require.NoError(t, svcCtx.DB.Where("username = ?", "sync_user_v8").First(&dbAdmin).Error)
	assert.Equal(t, 1, dbAdmin.Status)
	require.NoError(t, svcCtx.DB.Model(&dbAdmin).Update("status", 0).Error)
	svcCtx.AdminManager = NewAdminManager(dir)
	require.NoError(t, svcCtx.AdminManager.Initialize())
	err = seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
	require.NoError(t, svcCtx.DB.Where("username = ?", "sync_user_v8").First(&dbAdmin).Error)
	assert.Equal(t, 1, dbAdmin.Status)
}

func TestSeedBootstrapAdmins_RoleAssignmentV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	role := &model.Role{Name: "test_role_v8", Description: "Test Role V8"}
	require.NoError(t, svcCtx.DB.Create(role).Error)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"role_user_v8","password":"pass","roles":["test_role_v8"]}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)

	var admin model.Admin
	require.NoError(t, svcCtx.DB.Where("username = ?", "role_user_v8").First(&admin).Error)
	roles, err := svcCtx.AdminModel.GetAdminRoles(context.Background(), admin.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
}

func TestSeedBootstrapAdmins_RoleAlreadyAssignedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	role := &model.Role{Name: "existing_role_v8", Description: "Existing Role V8"}
	require.NoError(t, svcCtx.DB.Create(role).Error)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"dup_role_v8","password":"pass","roles":["existing_role_v8"]}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
	err = seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapAdmins_EmptyUsernameSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"","password":"p"},{"username":"valid_v8","password":"v"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapAdmins_EmptyPasswordSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"nopass_v8","password":""}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapAdmins(svcCtx)
	require.NoError(t, err)
}

// --- service_context.go: seedBootstrapRoles edge cases ---

func TestSeedBootstrapRoles_NilCtxV8(t *testing.T) {
	err := seedBootstrapRoles(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapRoles_EmptyRolesV8(t *testing.T) {
	dir := t.TempDir()
	svcCtx := &ServiceContext{
		AdminManager: NewAdminManager(dir),
		RoleModel:    model.NewRoleModel(newSvcTestContext(t).DB),
		DB:           newSvcTestContext(t).DB,
	}
	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapRoles_NoRoleModelV8(t *testing.T) {
	svcCtx := &ServiceContext{
		AdminManager: NewAdminManager(t.TempDir()),
	}
	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapRoles_WithPermissionsV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"test_role_v8","name":"Test Role","description":"test","permissions":["game:read"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[{"code":"game:read","name":"Read Game","description":"read game","category":"game","module":"game"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)

	var role model.Role
	require.NoError(t, svcCtx.DB.Where("name = ?", "test_role_v8").First(&role).Error)
}

func TestSeedBootstrapRoles_PermissionsAlreadyMatchV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"match_role_v8","name":"Match Role","permissions":["game:read"]}]`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[{"code":"game:read","name":"Read Game","description":"","category":"game","module":"game"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
	err = seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapRoles_NoPermissionsV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"noperm_role_v8","name":"No Perm Role"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapRoles_EmptyCodeSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[{"code":"","name":"Empty"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapRoles(svcCtx)
	require.NoError(t, err)
}

// --- service_context.go: seedBootstrapPermissions edge cases ---

func TestSeedBootstrapPermissions_NilCtxV8(t *testing.T) {
	err := seedBootstrapPermissions(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapPermissions_EmptyPermissionsV8(t *testing.T) {
	dir := t.TempDir()
	svcCtx := &ServiceContext{
		AdminManager:    NewAdminManager(dir),
		PermissionModel: model.NewPermissionModel(newSvcTestContext(t).DB),
		DB:              newSvcTestContext(t).DB,
	}
	err := seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapPermissions_NoPermissionModelV8(t *testing.T) {
	svcCtx := &ServiceContext{
		AdminManager: NewAdminManager(t.TempDir()),
	}
	err := seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapPermissions_NilPermV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.AdminManager = NewAdminManager(t.TempDir())
	svcCtx.AdminManager.permissions["nil_perm"] = nil
	err := seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapPermissions_EmptyCodeSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[{"code":"","name":"Empty"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapPermissions_ExistingPermSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[{"code":"existing_perm_v8","name":"Existing","description":"d","category":"cat","module":"mod"}]`), 0o644))
	svcCtx.AdminManager = NewAdminManager(dir)
	svcCtx.PermissionModel = model.NewPermissionModel(svcCtx.DB)
	require.NoError(t, svcCtx.AdminManager.Initialize())

	err := seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
	err = seedBootstrapPermissions(svcCtx)
	require.NoError(t, err)
}

// --- service_context.go: seedBootstrapTermDictionary edge cases ---

func TestSeedBootstrapTermDictionary_NilCtxV8(t *testing.T) {
	err := seedBootstrapTermDictionary(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapTermDictionary_NoModelV8(t *testing.T) {
	err := seedBootstrapTermDictionary(&ServiceContext{})
	require.NoError(t, err)
}

// --- service_context.go: seedBootstrapGames edge cases ---

func TestSeedBootstrapGames_NilCtxV8(t *testing.T) {
	err := seedBootstrapGames(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapGames_NoGameModelV8(t *testing.T) {
	err := seedBootstrapGames(&ServiceContext{AdminManager: NewAdminManager(t.TempDir())})
	require.NoError(t, err)
}

// --- service_context.go: other edge cases ---

func TestDefaultTermDictionaryConfigExtendedV8(t *testing.T) {
	cfg := defaultTermDictionaryConfig()
	assert.NotEmpty(t, cfg.Items)
	for _, item := range cfg.Items {
		assert.NotEmpty(t, item.Domain)
		assert.NotEmpty(t, item.Key)
	}
}

// --- AdminManager concurrent access ---

func TestAdminManager_ConcurrentAccessV8(t *testing.T) {
	am := NewAdminManager(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			username := "user" + string(rune('0'+n))
			_ = am.CreateAdmin(&AdminUser{Username: username, Password: "pass"})
			_, _ = am.GetAdmin(username)
			_ = am.ListAdmins()
		}(i)
	}
	wg.Wait()
}

// --- game_seed.go: additional edge cases ---

func TestSeedBootstrapGames_AlreadyHasGamesV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	game := &model.Game{GameID: "existing_game", Name: "Existing Game", AliasName: "Existing"}
	require.NoError(t, svcCtx.GameModel.Create(context.Background(), game))

	err := seedBootstrapGames(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapGames_NoGameModelV8b(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.GameModel = nil
	err := seedBootstrapGames(svcCtx)
	require.NoError(t, err)
}

func TestBuildGameFromSeed_EmptyGameIDV8(t *testing.T) {
	entry := bootstrapGameSeedEntry{}
	_, err := buildGameFromSeed(entry, fallbackDefaultEnvs, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game_id is required")
}

func TestBuildGameFromSeed_WithEnvV8(t *testing.T) {
	entry := bootstrapGameSeedEntry{
		GameID:  "test_game",
		Name:    "Test Game",
		Status:  "dev",
		Enabled: boolPtr(true),
		Env:     "prod",
	}
	game, err := buildGameFromSeed(entry, fallbackDefaultEnvs, 0)
	require.NoError(t, err)
	assert.Equal(t, "test_game", game.GameID)
	assert.True(t, game.Enabled)
}

func TestBuildGameFromSeed_WithEnvsV8(t *testing.T) {
	entry := bootstrapGameSeedEntry{
		GameID:  "test_game2",
		Name:    "Test Game 2",
		Status:  "prod",
		Enabled: boolPtr(false),
		Envs:    []model.GameEnv{{Env: "dev", Description: "Dev"}, {Env: "prod", Description: "Prod"}},
	}
	game, err := buildGameFromSeed(entry, fallbackDefaultEnvs, 5)
	require.NoError(t, err)
	assert.False(t, game.Enabled)
}

func TestBuildGameFromSeed_DefaultStatusV8(t *testing.T) {
	entry := bootstrapGameSeedEntry{
		GameID: "default_status_game",
		Name:   "Default Status",
	}
	game, err := buildGameFromSeed(entry, fallbackDefaultEnvs, 0)
	require.NoError(t, err)
	assert.Equal(t, "dev", game.Status)
}

func TestEnsureSeedEnvs_NilV8(t *testing.T) {
	entry := bootstrapGameSeedEntry{}
	result := ensureSeedEnvs(entry, fallbackDefaultEnvs)
	assert.Nil(t, result)
}

func TestResolveGamesConfigPath_BaseEmptyV8(t *testing.T) {
	path := resolveGamesConfigPath(config.Config{})
	assert.Equal(t, "", path)
}

func TestDefaultBootstrapGamesConfigV8(t *testing.T) {
	cfg := defaultBootstrapGamesConfig()
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.DefaultEnvs)
	assert.Len(t, cfg.Games, 1)
}

func TestPickGameColor_IndexV8(t *testing.T) {
	c := pickGameColor(0)
	assert.Equal(t, fallbackGameColorCycle[0], c)
	c2 := pickGameColor(len(fallbackGameColorCycle) + 1)
	assert.NotEmpty(t, c2)
}

func TestNormalizeColor_NoHashV8(t *testing.T) {
	result := normalizeColor("red", "#000")
	assert.Equal(t, "red", result)
}

func TestNormalizeColor_WithHashV8(t *testing.T) {
	result := normalizeColor("#FF0000", "#000")
	assert.Equal(t, "#ff0000", result)
}

func TestNormalizeColor_EmptyV8(t *testing.T) {
	result := normalizeColor("", "#000")
	assert.Equal(t, "#000", result)
}

func TestDefaultEnvColor_KnownV8(t *testing.T) {
	c := defaultEnvColor("prod")
	assert.Equal(t, "#13c2c2", c)
	c2 := defaultEnvColor("unknown_env")
	assert.Equal(t, defaultGameColor, c2)
}

func TestPickEnvColor_FromDefaultV8(t *testing.T) {
	defaults := map[string]model.GameEnv{
		"prod": {Env: "prod", Color: "#custom"},
	}
	c := pickEnvColor("prod", defaults)
	assert.Equal(t, "#custom", c)
}

func TestPickEnvColor_FallbackV8(t *testing.T) {
	c := pickEnvColor("sandbox", nil)
	assert.Equal(t, "#2f54eb", c)
}

func TestPickEnvColor_DefaultColorV8(t *testing.T) {
	c := pickEnvColor("unknown", nil)
	assert.Equal(t, defaultGameColor, c)
}

func TestSeedBootstrapExtensionCatalog_NilCtxV8(t *testing.T) {
	err := seedBootstrapExtensionCatalog(nil)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_NoDBV8(t *testing.T) {
	err := seedBootstrapExtensionCatalog(&ServiceContext{})
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_NoFileV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: t.TempDir()}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_BadJSONV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(`{bad`), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.Error(t, err)
}

func TestSeedBootstrapExtensionCatalog_EmptyItemsV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(`{"items":[]}`), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_WithReleasesV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	catalog := `{"items":[{"extensionId":"test-ext","name":"Test","displayName":"Test Ext","vendor":"v","kind":"plugin","summary":"s","status":"active","latestVersion":"1.0.0","releases":[{"version":"1.0.0","releaseChannel":"stable","publishedAt":"2024-01-01T00:00:00Z","manifest":{"id":"test-ext"}}]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(catalog), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_UpdateExistingV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	catalog := `{"items":[{"extensionId":"ext1","name":"Ext1","displayName":"Ext1 Display","vendor":"v","kind":"plugin","summary":"s","status":"active","latestVersion":"1.0.0","releases":[{"version":"1.0.0","releaseChannel":"stable","publishedAt":"2024-01-01T00:00:00Z"}]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(catalog), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
	err = seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_EmptyExtIDSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	catalog := `{"items":[{"extensionId":"","name":"Empty"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(catalog), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_EmptyVersionSkippedV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	catalog := `{"items":[{"extensionId":"ext2","name":"Ext2","releases":[{"version":"","releaseChannel":"stable"}]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(catalog), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}

func TestSeedBootstrapExtensionCatalog_EmptyManifestV8(t *testing.T) {
	svcCtx := newSvcTestContext(t)
	dir := t.TempDir()
	extDir := filepath.Join(dir, "extensions")
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	catalog := `{"items":[{"extensionId":"ext3","name":"Ext3","releases":[{"version":"1.0","releaseChannel":"","publishedAtUnix":0,"publishedAt":"","manifest":{}}]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "catalog.json"), []byte(catalog), 0o644))
	svcCtx.Config = config.Config{BootstrapData: config.BootstrapDataConfig{BaseDir: dir}}
	err := seedBootstrapExtensionCatalog(svcCtx)
	require.NoError(t, err)
}
