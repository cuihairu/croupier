package extension

import (
	"context"
	"github.com/cuihairu/croupier/internal/cache"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"sync"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

func boolPtr(b bool) *bool {
	return &b
}

var (
	integrationTestDB      *gorm.DB
	integrationTestDBOnce  sync.Once
	integrationTestDBMutex sync.Mutex
)

// setupIntegrationTestDB creates a shared in-memory SQLite database for integration testing

// setupIntegrationTestDB creates a shared in-memory SQLite database for integration testing
func setupIntegrationTestDB(t *testing.T) *gorm.DB {
	integrationTestDBMutex.Lock()
	defer integrationTestDBMutex.Unlock()

	integrationTestDBOnce.Do(func() {
		var err error
		integrationTestDB, err = gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		err = model.AutoMigrate(integrationTestDB)
		if err != nil {
			panic(err)
		}
	})

	// Clean up before each test
	integrationTestDB.Exec("DELETE FROM extension_installations")
	integrationTestDB.Exec("DELETE FROM extension_events")
	integrationTestDB.Exec("DELETE FROM extension_runtime_bindings")
	integrationTestDB.Exec("DELETE FROM extension_releases")
	integrationTestDB.Exec("DELETE FROM extension_catalogs")
	integrationTestDB.Exec("DELETE FROM admins")
	integrationTestDB.Exec("DELETE FROM roles")
	integrationTestDB.Exec("DELETE FROM admin_roles")
	integrationTestDB.Exec("DELETE FROM role_permissions")

	return integrationTestDB
}

// setupExtensionTestContext creates a test service context with extension services

// setupExtensionTestContext creates a test service context with extension services
func setupExtensionTestContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	extensionRepos := extensiongorm.NewBundle(db)
	extensionManifestSvc := extensionmanifest.NewService()
	extensionCatalogSvc := extensioncatalog.NewService(extensionRepos.Catalog, extensionRepos.Release)
	extensionInstallationSvc := extensioninstallation.NewService(extensionRepos.Installation, extensionRepos.Event, extensionRepos.Binding)
	extensionRuntimeSvc := extensionruntime.NewService(extensionRepos.Installation, extensionRepos.Release, extensionRepos.Binding, extensionRepos.Event)
	extensionSyncSvc := extensionsync.NewService(extensionRepos.Installation, extensionRepos.Binding)

	return &svc.ServiceContext{
		DB:                db,
		PermissionService: permSvc,
		Cache:             nullCache,
		CacheHelper:       cacheHelper,
		AdminModel:        model.NewAdminModel(db),
		RoleModel:         model.NewRoleModel(db),
		PermissionModel:   model.NewPermissionModel(db),
		RegistryStore:     reg.NewStore(),
		Dispatcher:        dispatch.NewDispatcher(reg.NewStore()),
		Extensions: &svc.ExtensionServices{
			Catalog:      extensionCatalogSvc,
			Manifest:     extensionManifestSvc,
			Installation: extensionInstallationSvc,
			Runtime:      extensionRuntimeSvc,
			Sync:         extensionSyncSvc,
		},
	}
}

// setupAdminContext creates an admin user with admin:all permission and returns context with username

// setupAdminContext creates an admin user with admin:all permission and returns context with username
func setupAdminContext(t *testing.T, svcCtx *svc.ServiceContext) context.Context {
	bg := context.Background()

	// Create admin role
	role := &model.Role{Name: "admin_role", Description: "Admin role for testing"}
	err := svcCtx.RoleModel.Create(bg, role)
	if err != nil {
		t.Fatalf("failed to create role: %v", err)
	}

	// Create admin
	admin := &model.Admin{Username: "test_admin", Status: 1}
	err = svcCtx.AdminModel.Create(bg, admin, "password123")
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	// Assign role to admin
	err = svcCtx.AdminModel.AssignRole(bg, admin.ID, role.ID)
	if err != nil {
		t.Fatalf("failed to assign role: %v", err)
	}

	// Grant admin:all permission to role
	err = svcCtx.RoleModel.ReplacePermissions(bg, role.ID, []string{"admin:all"})
	if err != nil {
		t.Fatalf("failed to grant permissions: %v", err)
	}

	return context.WithValue(bg, "username", "test_admin")
}

// createTestCatalogData creates test extension catalog and releases

// createTestCatalogData creates test extension catalog and releases
func createTestCatalogData(t *testing.T, svcCtx *svc.ServiceContext) {
	db := svcCtx.DB
	bg := context.Background()

	// Create test extension catalog
	catalog := &model.ExtensionCatalog{
		ExtensionID:   "test.analytics",
		Name:          "analytics",
		DisplayName:   "Test Analytics",
		Vendor:        "test",
		Kind:          "official",
		Summary:       "Test analytics extension",
		Status:        "active",
		LatestVersion: "2.0.0",
	}
	err := db.WithContext(bg).Create(catalog).Error
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}

	// Create another catalog entry
	catalog2 := &model.ExtensionCatalog{
		ExtensionID:   "test.notifications",
		Name:          "notifications",
		DisplayName:   "Test Notifications",
		Vendor:        "test",
		Kind:          "official",
		Summary:       "Test notifications extension",
		Status:        "active",
		LatestVersion: "1.5.0",
	}
	err = db.WithContext(bg).Create(catalog2).Error
	if err != nil {
		t.Fatalf("failed to create catalog2: %v", err)
	}

	// Create releases for test.analytics
	release1 := &model.ExtensionRelease{
		ExtensionID:     "test.analytics",
		Version:         "2.0.0",
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		Changelog:       "Initial release",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(`{"id":"test.analytics","version":"2.0.0","capabilities":["analytics.read","analytics.write"],"config_schema":{"type":"object","properties":{"enabled":{"type":"boolean"},"interval":{"type":"number"}},"required":["enabled"]},"default_install":true,"tags":["analytics","reporting"]}`)),
	}
	err = db.WithContext(bg).Create(release1).Error
	if err != nil {
		t.Fatalf("failed to create release1: %v", err)
	}

	release2 := &model.ExtensionRelease{
		ExtensionID:     "test.analytics",
		Version:         "1.0.0",
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		Changelog:       "First version",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(`{"id":"test.analytics","version":"1.0.0","capabilities":["analytics.read"]}`)),
	}
	err = db.WithContext(bg).Create(release2).Error
	if err != nil {
		t.Fatalf("failed to create release2: %v", err)
	}

	// Create release for test.notifications
	release3 := &model.ExtensionRelease{
		ExtensionID:     "test.notifications",
		Version:         "1.5.0",
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		Changelog:       "Notification extension",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(`{"id":"test.notifications","version":"1.5.0","capabilities":["notifications.send"]}`)),
	}
	err = db.WithContext(bg).Create(release3).Error
	if err != nil {
		t.Fatalf("failed to create release3: %v", err)
	}

	// Create catalog entry with dependencies
	catalogWithDeps := &model.ExtensionCatalog{
		ExtensionID:   "test.dependent",
		Name:          "dependent",
		DisplayName:   "Test Dependent Extension",
		Vendor:        "test",
		Kind:          "official",
		Summary:       "Extension with dependencies",
		Status:        "active",
		LatestVersion: "1.0.0",
	}
	err = db.WithContext(bg).Create(catalogWithDeps).Error
	if err != nil {
		t.Fatalf("failed to create catalogWithDeps: %v", err)
	}

	releaseWithDeps := &model.ExtensionRelease{
		ExtensionID:     "test.dependent",
		Version:         "1.0.0",
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		Changelog:       "Dependent extension",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(`{"id":"test.dependent","version":"1.0.0","dependencies":["test.analytics"]}`)),
	}
	err = db.WithContext(bg).Create(releaseWithDeps).Error
	if err != nil {
		t.Fatalf("failed to create releaseWithDeps: %v", err)
	}
}

// TestService_CatalogList_Success tests successful catalog list retrieval

// detailItemID: a test helper that fetches the current installation instance
// ID of the extension through the installation list.
func detailItemID(t *testing.T, s *Service, ctx context.Context, extensionID string) uint {
	t.Helper()
	resp, err := s.InstallationList(ctx, ExtensionInstallationListRequest{
		ExtensionID: extensionID,
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("installation list: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Fatal("expected at least one installation")
	}
	return resp.Items[0].ID
}

// TestService_CatalogWrite_PermissionDenied: a ctx without write permission is
// rejected (wiring verification of the utils-layer guard semantics in this
// package).
