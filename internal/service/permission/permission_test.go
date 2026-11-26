package permission

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ---------- helpers ----------

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Admin{},
		&model.Role{},
		&model.Permission{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// Create join tables that AutoMigrate won't handle
	db.Exec("CREATE TABLE IF NOT EXISTS admin_roles (admin_id INTEGER, role_id INTEGER)")
	db.Exec("CREATE TABLE IF NOT EXISTS role_permissions (role_id INTEGER, permission_id TEXT)")
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	admin := model.Admin{Username: "testadmin"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	role := model.Role{Name: "test_role"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	perm := model.Permission{
		ID:       "perm_read",
		Name:     "test permission",
		Resource: "game",
		Action:   "read",
	}
	if err := db.Create(&perm).Error; err != nil {
		t.Fatalf("seed permission: %v", err)
	}
}

// ---------- service.go tests ----------

func TestNewPermissionService(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	if svc == nil {
		t.Fatal("NewPermissionService returned nil")
	}
	if svc.db != db {
		t.Fatal("db not set correctly")
	}
}

func TestCheckPermission_ZeroAdminID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckPermission(context.Background(), 0, "game", "read")
	if !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("expected ErrAdminNotFound, got %v", err)
	}
}

func TestCheckPermission_InvalidResource(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckPermission(context.Background(), 1, "nonexistent", "read")
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
}

func TestCheckPermission_InvalidAction(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckPermission(context.Background(), 1, "game", "nonexistent")
	if !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("expected ErrInvalidAction, got %v", err)
	}
}

func TestCheckPermission_NoRoles(t *testing.T) {
	db := setupTestDB(t)
	// Create admin with no roles
	admin := model.Admin{Username: "noroleadmin"}
	db.Create(&admin)

	svc := NewPermissionService(db)
	ok, err := svc.CheckPermission(context.Background(), admin.ID, "game", "read")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got err=%v", err)
	}
	if ok {
		t.Fatal("expected false when no roles")
	}
}

func TestCheckPermission_WithRoles(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	// Create admin-role association
	var admin model.Admin
	db.Where("username = ?", "testadmin").First(&admin)
	var role model.Role
	db.Where("name = ?", "test_role").First(&role)

	// Create role_permission mapping — permission_id must be in "resource:action" format for casbin
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)
	db.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)",
		role.ID, "game:read")

	svc := NewPermissionService(db)
	ok, err := svc.CheckPermission(context.Background(), admin.ID, "game", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true when admin has role with matching permission")
	}
}

func TestCheckPermission_SuperAdminRole(t *testing.T) {
	db := setupTestDB(t)
	superAdmin := model.Admin{Username: "superadmin"}
	db.Create(&superAdmin)
	superRole := model.Role{Name: "super_admin"}
	db.Create(&superRole)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		superAdmin.ID, superRole.ID)

	svc := NewPermissionService(db)
	ok, err := svc.CheckPermission(context.Background(), superAdmin.ID, "game", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for super_admin role (has wildcard)")
	}
}

func TestHasAdminRole(t *testing.T) {
	tests := []struct {
		name  string
		roles []model.Role
		want  bool
	}{
		{"empty", nil, false},
		{"no admin", []model.Role{{Name: "viewer"}}, false},
		{"admin", []model.Role{{Name: "admin"}}, true},
		{"super_admin", []model.Role{{Name: "super_admin"}}, true},
		{"case insensitive", []model.Role{{Name: "Admin"}}, true},
		{"with spaces", []model.Role{{Name: "  super_admin  "}}, true},
		{"mixed", []model.Role{{Name: "viewer"}, {Name: "admin"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAdminRole(tt.roles)
			if got != tt.want {
				t.Errorf("hasAdminRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckGameScope_ZeroAdminID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckGameScope(context.Background(), 0, 1)
	if !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("expected ErrAdminNotFound, got %v", err)
	}
}

func TestCheckGameScope_Found(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_scopes (admin_id INTEGER, game_id INTEGER)")
	db.Exec("INSERT INTO admin_game_scopes (admin_id, game_id) VALUES (1, 10)")

	svc := NewPermissionService(db)
	ok, err := svc.CheckGameScope(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for existing scope")
	}
}

func TestCheckGameScope_NotFound(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_scopes (admin_id INTEGER, game_id INTEGER)")

	svc := NewPermissionService(db)
	ok, err := svc.CheckGameScope(context.Background(), 1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for non-existing scope")
	}
}

func TestCheckGameEnvScope_ZeroAdminID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckGameEnvScope(context.Background(), 0, 1, "prod")
	if !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("expected ErrAdminNotFound, got %v", err)
	}
}

func TestCheckGameEnvScope_Found(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")
	db.Exec("INSERT INTO admin_game_env_scopes (admin_id, game_id, env) VALUES (1, 10, 'prod')")

	svc := NewPermissionService(db)
	ok, err := svc.CheckGameEnvScope(context.Background(), 1, 10, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for existing env scope")
	}
}

func TestCheckGameEnvScope_NotFound(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")

	svc := NewPermissionService(db)
	ok, err := svc.CheckGameEnvScope(context.Background(), 1, 10, "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for non-existing env scope")
	}
}

func TestGetAdminPermissions(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	var admin model.Admin
	db.Where("username = ?", "testadmin").First(&admin)
	var role model.Role
	db.Where("name = ?", "test_role").First(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)
	db.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)",
		role.ID, "perm_read")

	svc := NewPermissionService(db)
	perms, err := svc.GetAdminPermissions(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}
	if perms[0].ID != "perm_read" {
		t.Fatalf("expected perm_read, got %s", perms[0].ID)
	}
}

func TestGetAdminPermissions_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	perms, err := svc.GetAdminPermissions(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != 0 {
		t.Fatalf("expected 0 permissions, got %d", len(perms))
	}
}

func TestGetAdminRoles(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	var admin model.Admin
	db.Where("username = ?", "testadmin").First(&admin)
	var role model.Role
	db.Where("name = ?", "test_role").First(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	roles, err := svc.GetAdminRoles(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(roles))
	}
	if roles[0].Name != "test_role" {
		t.Fatalf("expected test_role, got %s", roles[0].Name)
	}
}

func TestGetAdminRoles_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	roles, err := svc.GetAdminRoles(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected 0 roles, got %d", len(roles))
	}
}

func TestGetRoleIDs(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	roles := []model.Role{
		{Model: gorm.Model{ID: 1}},
		{Model: gorm.Model{ID: 2}},
		{Model: gorm.Model{ID: 3}},
	}
	ids := svc.getRoleIDs(roles)
	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(ids))
	}
	for i, want := range []uint{1, 2, 3} {
		if ids[i] != want {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], want)
		}
	}
}

func TestGetRoleIDs_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	ids := svc.getRoleIDs(nil)
	if len(ids) != 0 {
		t.Fatalf("expected empty, got %v", ids)
	}
}

func TestIsValidResource(t *testing.T) {
	validResources := []string{
		"admin", "role", "permission", "game", "player",
		"function", "component", "certificate", "backup",
		"analytics", "audit", "message", "ticket",
		"config", "schema", "provider", "pack",
		"workspace", "workspaces",
	}
	for _, r := range validResources {
		if !isValidResource(r) {
			t.Errorf("isValidResource(%q) = false, want true", r)
		}
	}
	invalid := []string{"", "foo", "bar", "123", "game "}
	for _, r := range invalid {
		if isValidResource(r) {
			t.Errorf("isValidResource(%q) = true, want false", r)
		}
	}
}

func TestIsValidAction(t *testing.T) {
	validActions := []string{
		"create", "read", "update", "edit", "delete", "execute",
		"publish", "install", "uninstall", "enable", "disable",
		"start", "stop", "restart", "approve", "reject", "rollback",
	}
	for _, a := range validActions {
		if !isValidAction(a) {
			t.Errorf("isValidAction(%q) = false, want true", a)
		}
	}
	invalid := []string{"", "foo", "bar", "123"}
	for _, a := range invalid {
		if isValidAction(a) {
			t.Errorf("isValidAction(%q) = true, want false", a)
		}
	}
}

func TestPermissionCandidates(t *testing.T) {
	tests := []struct {
		resource string
		action   string
		wantMin  int
	}{
		{"game", "read", 2},   // game:read + games:read + games:manage
		{"game", "create", 2}, // game:create + games:manage
		{"admin", "read", 2},  // admin:read + user:read + user:write
		{"admin", "create", 2},
		{"role", "read", 4},   // role:read + roles:read + role:write + roles:manage
		{"role", "create", 3}, // role:create + role:write + roles:manage
		{"permission", "read", 2},
		{"permission", "create", 2},
		{"function", "execute", 2}, // function:execute + function:invoke
		{"function", "read", 1},    // only function:read
		{"player", "read", 1},      // no special candidates
		{"", "read", 0},
		{"game", "", 0},
		{"", "", 0},
	}
	for _, tt := range tests {
		got := permissionCandidates(tt.resource, tt.action)
		if len(got) < tt.wantMin {
			t.Errorf("permissionCandidates(%q, %q) returned %d items, want at least %d: %v",
				tt.resource, tt.action, len(got), tt.wantMin, got)
		}
	}
}

func TestAppendUniquePermissionIDs(t *testing.T) {
	existing := []string{"perm1", "perm2"}
	result := appendUniquePermissionIDs(existing, "perm1", "perm3", "PERM3")
	if len(result) != 3 {
		t.Errorf("expected 3 unique ids, got %d: %v", len(result), result)
	}
	seen := make(map[string]bool)
	for _, r := range result {
		if seen[r] {
			t.Errorf("duplicate: %s", r)
		}
		seen[r] = true
	}
}

func TestUniqueLowered(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"dedup", []string{"A", "a", "B", "b"}, []string{"a", "b"}},
		{"spaces", []string{"  Foo  ", "FOO"}, []string{"foo"}},
		{"empty strings", []string{"", "  ", "a"}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueLowered(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tt.expect), got)
			}
			for i, w := range tt.expect {
				if got[i] != w {
					t.Errorf("got[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestLoadAdminAuthorizationState_WithRoles(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	var admin model.Admin
	db.Where("username = ?", "testadmin").First(&admin)
	var role model.Role
	db.Where("name = ?", "test_role").First(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)
	db.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)",
		role.ID, "perm_read")

	svc := NewPermissionService(db)
	roles, permIDs, err := svc.loadAdminAuthorizationState(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(roles))
	}
	if len(permIDs) < 1 {
		t.Fatalf("expected at least 1 permission id, got %d", len(permIDs))
	}
}

func TestLoadAdminAuthorizationState_AdminRole(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "adm"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	roles, permIDs, err := svc.loadAdminAuthorizationState(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAdminRole(roles) {
		t.Fatal("expected admin role")
	}
	found := false
	for _, pid := range permIDs {
		if pid == "admin:all" || pid == "*" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected admin:all or * in permission IDs, got %v", permIDs)
	}
}

func TestLoadAdminAuthorizationState_NoRoles(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	roles, permIDs, err := svc.loadAdminAuthorizationState(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected 0 roles, got %d", len(roles))
	}
	if len(permIDs) != 0 {
		t.Fatalf("expected 0 permission IDs, got %d", len(permIDs))
	}
}

// ---------- loadAdminAuthorizationState error paths ----------

func TestLoadAdminAuthorizationState_DBError(t *testing.T) {
	// Use a fresh DB with no admin_roles table to trigger error
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.Permission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// Do NOT create admin_roles table

	svc := NewPermissionService(db)
	_, _, loadErr := svc.loadAdminAuthorizationState(context.Background(), 1)
	if loadErr == nil {
		t.Fatal("expected error from DB")
	}
}

// ---------- CheckPermission with no matching permission ----------

func TestCheckPermission_NoMatchingPermission(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	var admin model.Admin
	db.Where("username = ?", "testadmin").First(&admin)
	var role model.Role
	db.Where("name = ?", "test_role").First(&role)
	// No role_permissions inserted → admin has role but no permission IDs
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	ok, err := svc.CheckPermission(context.Background(), admin.ID, "game", "read")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false when no permission IDs match")
	}
}

// ---------- CheckGameScope with DB error ----------

func TestCheckGameScope_DBError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	// Table doesn't exist → error
	_, err := svc.CheckGameScope(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("expected DB error")
	}
}

func TestCheckGameEnvScope_DBError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.CheckGameEnvScope(context.Background(), 1, 1, "prod")
	if err == nil {
		t.Fatal("expected DB error")
	}
}

// ---------- GetAdminPermissions error paths ----------

func TestGetAdminPermissions_JoinTableMissing(t *testing.T) {
	// Use a fresh DB without role_permissions/admin_roles tables
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.Permission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// Do NOT create role_permissions/admin_roles tables

	svc := NewPermissionService(db)
	_, queryErr := svc.GetAdminPermissions(context.Background(), 1)
	if queryErr == nil {
		t.Fatal("expected DB error")
	}
}

func TestGetAdminRoles_TableMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Admin{}, &model.Role{}, &model.Permission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// Do NOT create admin_roles table

	svc := NewPermissionService(db)
	_, queryErr := svc.GetAdminRoles(context.Background(), 1)
	if queryErr == nil {
		t.Fatal("expected DB error")
	}
}

// ---------- AdminRole case variations ----------

func TestHasAdminRole_CaseVariations(t *testing.T) {
	if !hasAdminRole([]model.Role{{Name: "ADMIN"}}) {
		t.Error("ADMIN should be recognized")
	}
	if !hasAdminRole([]model.Role{{Name: "Super_Admin"}}) {
		t.Error("Super_Admin should be recognized")
	}
	if hasAdminRole([]model.Role{{Name: "superadmin"}}) {
		t.Error("superadmin (no underscore) should not be recognized")
	}
}

// ---------- PermissionService with no matching permissions ----------

func TestCheckPermission_NoWildcardPerm(t *testing.T) {
	db := setupTestDB(t)
	// Create admin with a non-admin role that has no matching permission
	admin := model.Admin{Username: "dberr"}
	db.Create(&admin)
	role := model.Role{Name: "viewer"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)
	// Add a non-matching permission
	db.Exec("INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)",
		role.ID, "config:read")

	svc := NewPermissionService(db)
	// game:read won't match config:read
	ok, permErr := svc.CheckPermission(context.Background(), admin.ID, "game", "read")
	if permErr != nil {
		t.Fatalf("unexpected error: %v", permErr)
	}
	if ok {
		t.Fatal("expected false for non-matching permissions")
	}
}
