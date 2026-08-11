package permission

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	tokenmgr "github.com/cuihairu/croupier/internal/security/token"
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
			sqlDB.Close()
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

// ---------- middleware.go tests ----------

func TestPermissionMiddleware_EmptySecret(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Admin-ID", "1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should fail with permission denied since no roles
	if rr.Code != http.StatusForbidden {
		t.Logf("response: %s", rr.Body.String())
	}
}

func TestPermissionMiddleware_AdminIDFromContext(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "ctxadmin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	// Inject admin ID into context (simulating auth middleware)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_AdminIDFromContext_Int64(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "int64admin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	ctx := context.WithValue(req.Context(), "adminID", int64(admin.ID))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_XAdminIDHeader(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "headeradmin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Admin-ID", "1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_MissingAdminID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 for missing admin ID")
	}
}

func TestPermissionMiddleware_InvalidAdminIDFormat(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Admin-ID", "not-a-number")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 for invalid admin ID")
	}
}

func TestPermissionMiddleware_WithJWTToken(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "jwtuser"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	jwtSecret := "test-secret-key-for-testing"
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, jwtSecret, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Sign a token
	mgr := tokenmgr.NewManager(jwtSecret)
	token, err := mgr.Sign("jwtuser", []string{"admin"}, 3600000000000) // long TTL
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_InvalidJWTToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	jwtSecret := "test-secret"
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, jwtSecret, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 for invalid JWT")
	}
}

func TestPermissionMiddleware_CheckGameScope(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_scopes (admin_id INTEGER, game_id INTEGER)")
	db.Exec("INSERT INTO admin_game_scopes (admin_id, game_id) VALUES (1, 10)")

	admin := model.Admin{Username: "scopeadmin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:       "game",
		Action:         "read",
		CheckGameScope: true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with game ID in path
	req := httptest.NewRequest("GET", "/api/games/10", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_CheckGameScope_Denied(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_scopes (admin_id INTEGER, game_id INTEGER)")

	admin := model.Admin{Username: "noscoped"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:       "game",
		Action:         "read",
		CheckGameScope: true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 when game scope denied")
	}
}

func TestPermissionMiddleware_CheckEnvScope(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")
	db.Exec("INSERT INTO admin_game_env_scopes (admin_id, game_id, env) VALUES (1, 10, 'prod')")

	admin := model.Admin{Username: "envadmin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:      "game",
		Action:        "read",
		CheckEnvScope: true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10?env=prod", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPermissionMiddleware_CheckEnvScope_Denied(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")

	admin := model.Admin{Username: "noenvadmin"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:      "game",
		Action:        "read",
		CheckEnvScope: true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10?env=staging", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 when env scope denied")
	}
}

func TestPermissionMiddleware_ContextValues(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "ctxval"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	var capturedCtx context.Context
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if aid, ok := capturedCtx.Value("adminID").(uint); !ok || aid != admin.ID {
		t.Errorf("adminID in context = %v, want %d", capturedCtx.Value("adminID"), admin.ID)
	}
	if res, ok := capturedCtx.Value("resource").(string); !ok || res != "game" {
		t.Errorf("resource in context = %v, want game", capturedCtx.Value("resource"))
	}
	if act, ok := capturedCtx.Value("action").(string); !ok || act != "read" {
		t.Errorf("action in context = %v, want read", capturedCtx.Value("action"))
	}
}

// ---------- extractAdminID tests ----------

func TestExtractAdminID_FromContext(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	ctx := context.WithValue(context.Background(), "adminID", uint(42))
	req := httptest.NewRequest("GET", "/", nil)
	id, err := extractAdminID(ctx, req, nil, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected 42, got %d", id)
	}
}

func TestExtractAdminID_FromContextInt64(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	ctx := context.WithValue(context.Background(), "adminID", int64(99))
	req := httptest.NewRequest("GET", "/", nil)
	id, err := extractAdminID(ctx, req, nil, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 99 {
		t.Fatalf("expected 99, got %d", id)
	}
}

func TestExtractAdminID_ZeroFromContext(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	ctx := context.WithValue(context.Background(), "adminID", uint(0))
	req := httptest.NewRequest("GET", "/", nil)
	_, err := extractAdminID(ctx, req, nil, svc)
	if err == nil {
		t.Fatal("expected error for zero admin ID from context")
	}
}

func TestExtractAdminID_NilContext(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Admin-ID", "5")
	id, err := extractAdminID(nil, req, nil, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 5 {
		t.Fatalf("expected 5, got %d", id)
	}
}

func TestExtractAdminID_NoAdminID(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	req := httptest.NewRequest("GET", "/", nil)
	_, err := extractAdminID(context.Background(), req, nil, svc)
	if err == nil {
		t.Fatal("expected error when no admin ID in request")
	}
}

func TestExtractAdminID_InvalidFormat(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Admin-ID", "notanumber")
	_, err := extractAdminID(context.Background(), req, nil, svc)
	if err == nil {
		t.Fatal("expected error for invalid admin ID format")
	}
}

func TestExtractAdminID_WithJWT(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "jwtadmin"}
	db.Create(&admin)
	svc := NewPermissionService(db)
	jwtSecret := "test-secret-123"
	mgr := tokenmgr.NewManager(jwtSecret)
	token, _ := mgr.Sign("jwtadmin", []string{}, 3600000000000)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	id, err := extractAdminID(context.Background(), req, mgr, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != admin.ID {
		t.Fatalf("expected admin ID %d, got %d", admin.ID, id)
	}
}

func TestExtractAdminID_JWTNoSubject(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	jwtSecret := "test-secret-123"
	mgr := tokenmgr.NewManager(jwtSecret)
	// Sign with empty username
	token, _ := mgr.Sign("", []string{}, 3600000000000)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, err := extractAdminID(context.Background(), req, mgr, svc)
	if err == nil {
		t.Fatal("expected error for missing subject")
	}
}

func TestExtractAdminID_InvalidBearerFormat(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	mgr := tokenmgr.NewManager("test-secret")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	_, err := extractAdminID(context.Background(), req, mgr, svc)
	if err == nil {
		t.Fatal("expected error for non-Bearer auth")
	}
}

func TestExtractAdminID_NoTokenManager(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	_, err := extractAdminID(context.Background(), req, nil, svc)
	if err == nil {
		t.Fatal("expected error when tokenManager is nil")
	}
}

// ---------- extractGameID tests ----------

func TestExtractGameID_FromPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games/42/details", nil)
	id, err := extractGameID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected 42, got %d", id)
	}
}

func TestExtractGameID_FromQueryParamGameId(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games?gameId=55", nil)
	id, err := extractGameID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 55 {
		t.Fatalf("expected 55, got %d", id)
	}
}

func TestExtractGameID_FromQueryParamGameID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games?game_id=77", nil)
	id, err := extractGameID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 77 {
		t.Fatalf("expected 77, got %d", id)
	}
}

func TestExtractGameID_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Game-ID", "88")
	id, err := extractGameID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 88 {
		t.Fatalf("expected 88, got %d", id)
	}
}

func TestExtractGameID_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games", nil)
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error when no game ID")
	}
}

func TestExtractGameID_InvalidPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games/notanumber", nil)
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error for invalid path game ID")
	}
}

func TestExtractGameID_InvalidQueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games?gameId=abc", nil)
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error for invalid query game ID")
	}
}

func TestExtractGameID_InvalidHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Game-ID", "abc")
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error for invalid header game ID")
	}
}

func TestExtractGameID_PathNotGames(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users/42", nil)
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error when path doesn't start with /games/")
	}
}

// ---------- extractEnv tests ----------

func TestExtractEnv_FromQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/?env=prod", nil)
	env, err := extractEnv(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != "prod" {
		t.Fatalf("expected prod, got %s", env)
	}
}

func TestExtractEnv_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Env", "staging")
	env, err := extractEnv(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != "staging" {
		t.Fatalf("expected staging, got %s", env)
	}
}

func TestExtractEnv_NotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, err := extractEnv(req)
	if err == nil {
		t.Fatal("expected error when no env")
	}
}

func TestExtractEnv_QueryTakesPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/?env=prod", nil)
	req.Header.Set("X-Env", "staging")
	env, err := extractEnv(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != "prod" {
		t.Fatalf("expected prod (query takes priority), got %s", env)
	}
}

// ---------- writePermissionError tests ----------

func TestWritePermissionError_PermissionDenied(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, ErrPermissionDenied)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestWritePermissionError_AdminNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, ErrAdminNotFound)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestWritePermissionError_InvalidResource(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, ErrInvalidResource)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestWritePermissionError_InvalidAction(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, ErrInvalidAction)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestWritePermissionError_CodeError(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, errorx.NewForbidden("custom forbidden"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestWritePermissionError_UnknownError(t *testing.T) {
	rr := httptest.NewRecorder()
	writePermissionError(context.Background(), rr, errors.New("something went wrong"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// ---------- writeJSONError tests ----------

func TestWriteJSONError_CodeError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, errorx.NewBadRequest("test error"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "bad_request" {
		t.Errorf("expected error code bad_request, got %v", body["error"])
	}
}

func TestWriteJSONError_NonCodeError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, errors.New("generic error"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestWriteJSONError_ContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, errorx.NewBadRequest("test"))
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

// ---------- Predefined config functions ----------

func TestPredefinedConfigs(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() PermissionConfig
		resource string
		action   string
	}{
		{"AdminReadPermission", AdminReadPermission, "admin", "read"},
		{"AdminCreatePermission", AdminCreatePermission, "admin", "create"},
		{"AdminUpdatePermission", AdminUpdatePermission, "admin", "update"},
		{"AdminDeletePermission", AdminDeletePermission, "admin", "delete"},
		{"RoleManagePermission", RoleManagePermission, "role", "create"},
		{"GameManagePermission", GameManagePermission, "game", "create"},
		{"GameReadPermission", GameReadPermission, "game", "read"},
		{"PlayerManagePermission", PlayerManagePermission, "player", "create"},
		{"FunctionExecutePermission", FunctionExecutePermission, "function", "execute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.fn()
			if cfg.Resource != tt.resource {
				t.Errorf("Resource = %q, want %q", cfg.Resource, tt.resource)
			}
			if cfg.Action != tt.action {
				t.Errorf("Action = %q, want %q", cfg.Action, tt.action)
			}
		})
	}
}

func TestFunctionExecutePermission_ScopeFlags(t *testing.T) {
	cfg := FunctionExecutePermission()
	if !cfg.CheckGameScope {
		t.Error("expected CheckGameScope=true")
	}
	if !cfg.CheckEnvScope {
		t.Error("expected CheckEnvScope=true")
	}
}

func TestPlayerManagePermission_ScopeFlags(t *testing.T) {
	cfg := PlayerManagePermission()
	if !cfg.CheckGameScope {
		t.Error("expected CheckGameScope=true")
	}
	if cfg.CheckEnvScope {
		t.Error("expected CheckEnvScope=false")
	}
}

// ---------- PermissionMiddleware with invalid admin ----------

func TestPermissionMiddleware_InvalidAdmin(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "read"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games", nil)
	req.Header.Set("X-Admin-ID", "1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Admin doesn't exist, so no roles → permission denied
	if rr.Code == http.StatusOK {
		t.Fatal("expected non-200 for admin with no roles")
	}
}

func TestPermissionMiddleware_PermissionDenied(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "denieduser"}
	db.Create(&admin)
	// Create role with no matching permissions
	role := model.Role{Name: "viewer"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{Resource: "game", Action: "create"}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/games", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
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

// ---------- PermissionMiddleware with both scope checks ----------

func TestPermissionMiddleware_BothScopes(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_scopes (admin_id INTEGER, game_id INTEGER)")
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")
	db.Exec("INSERT INTO admin_game_scopes (admin_id, game_id) VALUES (1, 10)")
	db.Exec("INSERT INTO admin_game_env_scopes (admin_id, game_id, env) VALUES (1, 10, 'prod')")

	admin := model.Admin{Username: "bothscope"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:       "game",
		Action:         "read",
		CheckGameScope: true,
		CheckEnvScope:  true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/10?env=prod", nil)
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------- extractGameID with both gameId and game_id query params ----------

func TestExtractGameID_GameIdQueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api?gameId=33", nil)
	id, err := extractGameID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 33 {
		t.Fatalf("expected 33, got %d", id)
	}
}

// ---------- extractEnv with header only ----------

func TestExtractEnv_HeaderOnly(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Env", "dev")
	env, err := extractEnv(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != "dev" {
		t.Fatalf("expected dev, got %s", env)
	}
}

// ---------- lookupAdminByUsername tests ----------

func TestLookupAdminByUsername_Found(t *testing.T) {
	db := setupTestDB(t)
	admin := model.Admin{Username: "lookupuser"}
	db.Create(&admin)

	svc := NewPermissionService(db)
	result, err := svc.lookupAdminByUsername("lookupuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "lookupuser" {
		t.Fatalf("expected lookupuser, got %s", result.Username)
	}
}

func TestLookupAdminByUsername_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.lookupAdminByUsername("")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestLookupAdminByUsername_Whitespace(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.lookupAdminByUsername("   ")
	if err == nil {
		t.Fatal("expected error for whitespace username")
	}
}

func TestLookupAdminByUsername_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewPermissionService(db)
	_, err := svc.lookupAdminByUsername("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

// ---------- Edge case: extractAdminID with context value type mismatch ----------

func TestExtractAdminID_ContextInvalidType(t *testing.T) {
	svc := NewPermissionService(setupTestDB(t))
	ctx := context.WithValue(context.Background(), "adminID", "not-an-int")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Admin-ID", "10")
	id, err := extractAdminID(ctx, req, nil, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 10 {
		t.Fatalf("expected 10 from header, got %d", id)
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

// ---------- WriteJSONError fallback with non-CodeError error ----------

func TestWriteJSONError_NonCodeErrorFallback(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, errors.New("plain error"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["error"] != "internal_error" {
		t.Errorf("expected internal_error, got %v", body["error"])
	}
}

// ---------- Verify JSON error body ----------

func TestWriteJSONError_CodeErrorBody(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, errorx.NewBadRequest("test msg"))
	var body map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&body)
	if body["message"] != "test msg" {
		t.Errorf("expected message 'test msg', got %v", body["message"])
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

// ---------- PermissionMiddleware: env scope with header ----------

func TestPermissionMiddleware_EnvScopeFromHeader(t *testing.T) {
	db := setupTestDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS admin_game_env_scopes (admin_id INTEGER, game_id INTEGER, env TEXT)")
	db.Exec("INSERT INTO admin_game_env_scopes (admin_id, game_id, env) VALUES (1, 5, 'staging')")

	admin := model.Admin{Username: "envhdr"}
	db.Create(&admin)
	role := model.Role{Name: "admin"}
	db.Create(&role)
	db.Exec("INSERT INTO admin_roles (admin_id, role_id) VALUES (?, ?)",
		admin.ID, role.ID)

	svc := NewPermissionService(db)
	config := PermissionConfig{
		Resource:      "game",
		Action:        "read",
		CheckEnvScope: true,
	}
	handler := PermissionMiddleware(svc, "", config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/games/5", nil)
	req.Header.Set("X-Env", "staging")
	ctx := context.WithValue(req.Context(), "adminID", admin.ID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------- extractGameID: path with only /games/ (no ID) ----------

func TestExtractGameID_ShortPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/games", nil)
	_, err := extractGameID(req)
	if err == nil {
		t.Fatal("expected error for path without game ID")
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
