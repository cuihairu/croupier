package svc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminManagerLoadDefaultAdminsSetsDefaultStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	data := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	if err := os.WriteFile(filepath.Join(dir, "admins.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write admins.json failed: %v", err)
	}

	manager := NewAdminManager(dir)
	if err := manager.loadDefaultAdmins(); err != nil {
		t.Fatalf("loadDefaultAdmins failed: %v", err)
	}

	admin, err := manager.GetAdmin("admin")
	if err != nil {
		t.Fatalf("GetAdmin failed: %v", err)
	}
	if admin.Status != 1 {
		t.Fatalf("expected default status=1, got %d", admin.Status)
	}
	if admin.CreateAt == "" || admin.UpdateAt == "" {
		t.Fatal("expected default timestamps to be filled")
	}
}

func TestAdminManagerInitialize(t *testing.T) {
	dir := t.TempDir()

	// Create admins.json
	adminsData := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsData), 0o644))

	// Create roles.json
	rolesData := `[{"name":"admin","description":"Administrator","permissions":["*"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(rolesData), 0o644))

	// Create permissions.json
	permsData := `[{"code":"admin:read","resource":"admin","action":"read","description":"Read admin"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(permsData), 0o644))

	manager := NewAdminManager(dir)
	err := manager.Initialize()
	require.NoError(t, err)

	// Verify admin was created
	admin, err := manager.GetAdmin("admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", admin.Username)
}

func TestAdminManagerInitialize_NoConfigDir(t *testing.T) {
	manager := NewAdminManager("")
	err := manager.Initialize()
	require.NoError(t, err)
}

func TestAdminManagerInitialize_NonexistentDir(t *testing.T) {
	manager := NewAdminManager("/nonexistent/path/that/does/not/exist")
	err := manager.Initialize()
	require.NoError(t, err)
}

func TestAdminManagerLoadDefaultAdmins_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte("[]"), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultAdmins()
	require.NoError(t, err)
}

func TestAdminManagerLoadDefaultAdmins_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte("invalid"), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultAdmins()
	// The function continues even with invalid JSON, so it returns nil
	assert.NoError(t, err)
}

func TestAdminManagerLoadDefaultRoles(t *testing.T) {
	dir := t.TempDir()
	rolesData := `[{"name":"admin","description":"Administrator","permissions":["*"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(rolesData), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultRoles()
	require.NoError(t, err)
}

func TestAdminManagerLoadDefaultRoles_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte("[]"), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultRoles()
	require.NoError(t, err)
}

func TestAdminManagerLoadDefaultPermissions(t *testing.T) {
	dir := t.TempDir()
	permsData := `[{"code":"admin:read","resource":"admin","action":"read","description":"Read admin"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(permsData), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultPermissions()
	require.NoError(t, err)
}

func TestAdminManagerLoadDefaultPermissions_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "permissions.json"), []byte("[]"), 0o644))

	manager := NewAdminManager(dir)
	err := manager.loadDefaultPermissions()
	require.NoError(t, err)
}

func TestAdminManagerValidateUser(t *testing.T) {
	dir := t.TempDir()
	adminsData := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsData), 0o644))

	manager := NewAdminManager(dir)
	require.NoError(t, manager.loadDefaultAdmins())

	// Valid user
	admin, err := manager.ValidateUser("admin", "admin123")
	assert.NoError(t, err)
	assert.NotNil(t, admin)

	// Invalid password
	admin, err = manager.ValidateUser("admin", "wrongpassword")
	assert.Error(t, err)
	assert.Nil(t, admin)

	// Non-existent user
	admin, err = manager.ValidateUser("nonexistent", "password")
	assert.Error(t, err)
	assert.Nil(t, admin)
}

func TestAdminManagerCreateAdmin(t *testing.T) {
	dir := t.TempDir()
	manager := NewAdminManager(dir)

	admin := &AdminUser{
		Username: "newadmin",
		Password: "password123",
		Roles:    []string{"admin"},
	}
	err := manager.CreateAdmin(admin)
	require.NoError(t, err)

	created, err := manager.GetAdmin("newadmin")
	require.NoError(t, err)
	assert.Equal(t, "newadmin", created.Username)
}

func TestAdminManagerResetPassword(t *testing.T) {
	dir := t.TempDir()
	adminsData := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsData), 0o644))

	manager := NewAdminManager(dir)
	require.NoError(t, manager.loadDefaultAdmins())

	err := manager.ResetPassword("admin", "newpassword")
	require.NoError(t, err)

	// Verify new password works
	admin, err := manager.ValidateUser("admin", "newpassword")
	assert.NoError(t, err)
	assert.NotNil(t, admin)
}

func TestAdminManagerCheckPermission(t *testing.T) {
	dir := t.TempDir()
	adminsData := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsData), 0o644))

	rolesData := `[{"name":"admin","description":"Administrator","permissions":["admin:read","admin:write"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "roles.json"), []byte(rolesData), 0o644))

	manager := NewAdminManager(dir)
	require.NoError(t, manager.loadDefaultAdmins())
	require.NoError(t, manager.loadDefaultRoles())

	// Admin role has wildcard permission
	hasPermission := manager.CheckPermission("admin", "admin:read")
	assert.True(t, hasPermission)
}

func TestAdminManagerGetAdmin(t *testing.T) {
	dir := t.TempDir()
	adminsData := `[{"username":"admin","password":"admin123","roles":["admin"]}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsData), 0o644))

	manager := NewAdminManager(dir)
	require.NoError(t, manager.loadDefaultAdmins())

	// Existing admin
	admin, err := manager.GetAdmin("admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", admin.Username)

	// Non-existent admin
	admin, err = manager.GetAdmin("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, admin)
}
