package svc

import (
	"os"
	"path/filepath"
	"testing"
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
