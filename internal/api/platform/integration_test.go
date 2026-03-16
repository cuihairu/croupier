//go:build integration
// +build integration

package platform

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TestDiscoverExternalPlatforms_WithInstallationDB tests the installation bindings path with a real database
func TestDiscoverExternalPlatforms_WithInstallationDB(t *testing.T) {
	// Create in-memory SQLite database
	db, err := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	// Create repos
	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)

	// Create installation service
	installationService := installation.NewService(installationRepo, eventRepo, bindingRepo)

	// Create a test installation
	testInstallation := &model.ExtensionInstallation{
		InstallationKey: "test-external-platform",
		ExtensionID:     "test-external-platform",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "default",
		TargetType:      "server",
		TargetID:        "default",
		Status:          "installed",
		DesiredState:    "installed",
		Enabled:         true,
		InstalledBy:     "test",
		InstalledAtUnix: time.Now().Unix(),
	}
	if err := installationRepo.Create(context.Background(), testInstallation); err != nil {
		t.Fatalf("failed to create installation: %v", err)
	}

	// Create test bindings
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "steam",
			SpecJSON:    datatypes.JSON([]byte(`{"provider":"steam","operations":["get_player","ban_player"]}`)),
			Status:      "active",
		},
		{
			BindingType: "function",
			BindingKey:  "external.k8s.restart_pod",
			Status:      "active",
		},
	}
	if err := bindingRepo.ReplaceForInstallation(context.Background(), testInstallation.ID, bindings); err != nil {
		t.Fatalf("failed to create bindings: %v", err)
	}

	// Create service context with installation service
	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationService,
		},
	}
	service := NewService(svcCtx)

	// Test discoverExternalPlatforms
	platforms := service.discoverExternalPlatforms(context.Background())

	// Should discover platforms from installation bindings
	if len(platforms["steam"]) != 2 {
		t.Fatalf("expected 2 methods for steam platform, got %d: %v", len(platforms["steam"]), platforms["steam"])
	}
	if len(platforms["k8s"]) != 1 {
		t.Fatalf("expected 1 method for k8s platform, got %d: %v", len(platforms["k8s"]), platforms["k8s"])
	}

	// Verify methods
	if !contains(platforms["steam"], "get_player") {
		t.Fatal("expected get_player method for steam")
	}
	if !contains(platforms["steam"], "ban_player") {
		t.Fatal("expected ban_player method for steam")
	}
	if !contains(platforms["k8s"], "restart_pod") {
		t.Fatal("expected restart_pod method for k8s")
	}
}

// TestDiscoverExternalPlatforms_SkipsUninstalledWithDB tests that uninstalled extensions are skipped
func TestDiscoverExternalPlatforms_SkipsUninstalledWithDB(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)
	installationService := installation.NewService(installationRepo, eventRepo, bindingRepo)

	// Create installed extension
	installed := &model.ExtensionInstallation{
		InstallationKey: "installed-ext",
		ExtensionID:     "installed-external-platform",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "default",
		TargetType:      "server",
		TargetID:        "default",
		Status:          "installed",
		DesiredState:    "installed",
		Enabled:         true,
		InstalledBy:     "test",
		InstalledAtUnix: time.Now().Unix(),
	}
	if err := installationRepo.Create(context.Background(), installed); err != nil {
		t.Fatalf("failed to create installed installation: %v", err)
	}

	// Create uninstalled extension
	uninstalled := &model.ExtensionInstallation{
		InstallationKey: "uninstalled-ext",
		ExtensionID:     "uninstalled-external-platform",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "default",
		TargetType:      "server",
		TargetID:        "default",
		Status:          "uninstalled",
		DesiredState:    "uninstalled",
		Enabled:         false,
		InstalledBy:     "test",
		InstalledAtUnix: time.Now().Unix(),
	}
	if err := installationRepo.Create(context.Background(), uninstalled); err != nil {
		t.Fatalf("failed to create uninstalled installation: %v", err)
	}

	// Create binding for installed extension
	binding := []model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "test",
			SpecJSON:    datatypes.JSON([]byte(`{"provider":"test","operations":["method1"]}`)),
			Status:      "active",
		},
	}
	if err := bindingRepo.ReplaceForInstallation(context.Background(), installed.ID, binding); err != nil {
		t.Fatalf("failed to create binding: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{
			Installation: installationService,
		},
	}
	service := NewService(svcCtx)

	platforms := service.discoverExternalPlatforms(context.Background())

	// Should only include installed extension
	if _, ok := platforms["test"]; !ok {
		t.Fatal("expected test platform from installed extension")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
