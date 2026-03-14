package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSeedBootstrapAdminsRepairsExistingAdminRoleBindings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "admins.json"), []byte(`[{"username":"admin","password":"admin123","roles":["admin"]}]`), 0o644); err != nil {
		t.Fatalf("write admins.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "permissions.json"), []byte(`[
		{"code":"admin:all","name":"All","description":"all","category":"admin","module":"system"},
		{"code":"*","name":"Wildcard","description":"all","category":"admin","module":"system"}
	]`), 0o644); err != nil {
		t.Fatalf("write permissions.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roles.json"), []byte(`[
		{"code":"admin","name":"Admin","description":"admin role","level":100,"permissions":["admin:all","*"]}
	]`), 0o644); err != nil {
		t.Fatalf("write roles.json failed: %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	ctx := &ServiceContext{
		DB:              db,
		AdminManager:    NewAdminManager(dir),
		AdminModel:      model.NewAdminModel(db),
		RoleModel:       model.NewRoleModel(db),
		PermissionModel: model.NewPermissionModel(db),
	}
	if err := ctx.AdminManager.Initialize(); err != nil {
		t.Fatalf("admin manager init failed: %v", err)
	}

	// Simulate an existing migrated admin row with no admin_roles binding.
	existing := &model.Admin{
		Username: "admin",
		Status:   0,
	}
	if err := ctx.AdminModel.Create(context.Background(), existing, "admin123"); err != nil {
		t.Fatalf("create existing admin failed: %v", err)
	}

	if err := seedBootstrapPermissions(ctx); err != nil {
		t.Fatalf("seedBootstrapPermissions failed: %v", err)
	}
	if err := seedBootstrapRoles(ctx); err != nil {
		t.Fatalf("seedBootstrapRoles failed: %v", err)
	}
	if err := seedBootstrapAdmins(ctx); err != nil {
		t.Fatalf("seedBootstrapAdmins failed: %v", err)
	}

	admin, err := ctx.AdminModel.FindByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("FindByUsername failed: %v", err)
	}
	if admin.Status != 1 {
		t.Fatalf("expected bootstrap admin status=1, got %d", admin.Status)
	}

	roles, err := ctx.AdminModel.GetAdminRoles(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("GetAdminRoles failed: %v", err)
	}
	if len(roles) != 1 || roles[0].Name != "admin" {
		t.Fatalf("expected admin role binding, got %#v", roles)
	}

	permIDs, err := ctx.RoleModel.GetRolePermissionIDs(context.Background(), roles[0].ID)
	if err != nil {
		t.Fatalf("GetRolePermissionIDs failed: %v", err)
	}
	if len(permIDs) != 2 {
		t.Fatalf("expected 2 permissions, got %#v", permIDs)
	}
}
