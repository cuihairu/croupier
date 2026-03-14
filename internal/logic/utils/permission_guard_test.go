package utils

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRequireAnyPermissionUsesCasbinForBootstrapAdmin(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	adminModel := model.NewAdminModel(db)
	roleModel := model.NewRoleModel(db)

	admin := &model.Admin{Username: "admin", Status: 1}
	if err := adminModel.Create(context.Background(), admin, "admin123"); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	role := &model.Role{Name: "admin", Description: "admin"}
	if err := roleModel.Create(context.Background(), role); err != nil {
		t.Fatalf("create role failed: %v", err)
	}
	if err := adminModel.AssignRole(context.Background(), admin.ID, role.ID); err != nil {
		t.Fatalf("assign role failed: %v", err)
	}
	if err := roleModel.ReplacePermissions(context.Background(), role.ID, []string{"admin:all"}); err != nil {
		t.Fatalf("replace permissions failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		DB:              db,
		AdminModel:      adminModel,
		RoleModel:       roleModel,
		PermissionModel: model.NewPermissionModel(db),
	}

	ctx := context.WithValue(context.Background(), "username", "admin")
	if _, _, err := RequireAnyPermission(ctx, svcCtx, "forbidden", "user:read"); err != nil {
		t.Fatalf("RequireAnyPermission failed: %v", err)
	}
}
