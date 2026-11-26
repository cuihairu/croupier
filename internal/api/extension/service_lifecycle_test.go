package extension

import (
	"context"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

// TestService_Install_Success tests successful extension installation
func TestService_Install_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true, "interval": 60},
	}

	resp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InstallationID == 0 {
		t.Fatal("expected non-zero installation_id")
	}

	if resp.Status != "installed" {
		t.Fatalf("expected status 'installed', got: %s", resp.Status)
	}
}

func TestService_Install_RejectsUnsupportedScopeType(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "organization",
		ScopeID:        "org123",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true, "interval": 60},
	}

	_, err := s.Install(ctx, req, "test_admin")
	if err == nil {
		t.Fatal("expected unsupported scope_type error")
	}
	assert.Contains(t, err.Error(), "unsupported extension scope_type")
}

func TestService_Install_NormalizesScopeTypeAndID(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      " Game ",
		ScopeID:        " demo ",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true, "interval": 60},
	}

	resp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	detail, err := s.InstallationDetail(ctx, resp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected detail error: %v", err)
	}
	assert.Equal(t, "game", detail.Installation.ScopeType)
	assert.Equal(t, "demo", detail.Installation.ScopeID)
}

// TestService_Install_Conflict tests installation conflict detection

// TestService_Install_Conflict tests installation conflict detection
func TestService_Install_Conflict(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}

	// First installation should succeed
	_, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("first installation failed: %v", err)
	}

	// Second installation with same scope/target should conflict
	_, err = s.Install(ctx, req, "test_admin")
	if err == nil {
		t.Fatal("expected conflict error for duplicate installation")
	}
}

// TestService_Install_MissingRequiredConfig tests installation with missing required config

// TestService_Install_MissingRequiredConfig tests installation with missing required config
func TestService_Install_MissingRequiredConfig(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{}, // Missing required 'enabled' field
	}

	_, err := s.Install(ctx, req, "test_admin")
	if err == nil {
		t.Fatal("expected error for missing required config field")
	}
}

// TestService_InstallationList_Success tests successful installation list retrieval

// TestService_InstallationList_Success tests successful installation list retrieval
func TestService_InstallationList_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	_, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// List installations
	listReq := ExtensionInstallationListRequest{
		ExtensionID: "test.analytics",
		Page:        1,
		PageSize:    10,
	}

	resp, err := s.InstallationList(ctx, listReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Items) == 0 {
		t.Fatal("expected at least one installation")
	}
}

// TestService_InstallationDetail_Success tests successful installation detail retrieval

// TestService_InstallationDetail_Success tests successful installation detail retrieval
func TestService_InstallationDetail_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true, "interval": 60},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Get installation detail
	resp, err := s.InstallationDetail(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Installation == nil {
		t.Fatal("expected installation to be non-nil")
	}

	if resp.Installation.ExtensionID != "test.analytics" {
		t.Fatalf("expected extension_id test.analytics, got: %s", resp.Installation.ExtensionID)
	}

	if len(resp.ConfigSchema) == 0 {
		t.Fatal("expected config_schema from manifest")
	}

	if resp.Config == nil {
		t.Fatal("expected config to be non-nil")
	}

	if enabled, ok := resp.Config["enabled"].(bool); !ok || !enabled {
		t.Fatal("expected enabled=true in config")
	}
}

// TestService_UpdateConfig_Success tests successful config update

// TestService_Enable_Success tests enable operation
func TestService_Enable_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Enable the installation
	resp, err := s.Enable(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "enabled" {
		t.Fatalf("expected status 'enabled', got: %s", resp.Status)
	}
}

// TestService_Disable_Success tests disable operation

// TestService_Disable_Success tests disable operation
func TestService_Disable_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create and enable an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// First enable
	_, err = s.Enable(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Then disable
	resp, err := s.Disable(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "disabled" {
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}
}

// TestService_Upgrade_Success tests upgrade operation

// TestService_Upgrade_Success tests upgrade operation
func TestService_Upgrade_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation with version 1.0.0
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Upgrade to 2.0.0
	resp, err := s.Upgrade(ctx, installResp.InstallationID, "2.0.0", "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "upgraded" {
		t.Fatalf("expected status 'upgraded', got: %s", resp.Status)
	}
}

// TestService_Upgrade_AlreadyOnTargetVersion tests upgrade to same version

// TestService_Upgrade_AlreadyOnTargetVersion tests upgrade to same version
func TestService_Upgrade_AlreadyOnTargetVersion(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation with version 2.0.0
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Try to upgrade to same version
	_, err = s.Upgrade(ctx, installResp.InstallationID, "2.0.0", "test_admin")
	if err == nil {
		t.Fatal("expected error for upgrading to same version")
	}
}

// TestService_Upgrade_VersionNotFound tests upgrade to non-existent version

// TestService_Upgrade_VersionNotFound tests upgrade to non-existent version
func TestService_Upgrade_VersionNotFound(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Try to upgrade to non-existent version
	_, err = s.Upgrade(ctx, installResp.InstallationID, "99.99.99", "test_admin")
	if err == nil {
		t.Fatal("expected error for non-existent version")
	}
}

// TestService_Reconcile_Success tests reconcile operation

// TestService_Reconcile_Success tests reconcile operation
func TestService_Reconcile_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Reconcile
	resp, err := s.Reconcile(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Applied < 0 {
		t.Fatalf("expected applied >= 0, got: %d", resp.Applied)
	}
}

// TestService_Uninstall_Success tests uninstall operation

// TestService_Uninstall_Success tests uninstall operation
func TestService_Uninstall_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Uninstall
	resp, err := s.Uninstall(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "uninstalled" {
		t.Fatalf("expected status 'uninstalled', got: %s", resp.Status)
	}
}

// TestService_Events_Success tests events retrieval

// TestService_InstallationList_DisplayNameJoin_HealthDerived (#46 batch 2):
// list assembly joins the catalog display names (displayName -> name ->
// extensionID fallback) + healthStatus is derived from status/enabled
// (replacing the always-unknown value).
func TestService_InstallationList_DisplayNameJoin_HealthDerived(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)
	bg := context.Background()

	// A catalog row without displayName: verifies the fallback chain reaches
	// name.
	bare := &model.ExtensionCatalog{
		ExtensionID: "test.bare",
		Name:        "bare-name",
		Kind:        "official",
		Status:      "active",
	}
	if err := db.WithContext(bg).Create(bare).Error; err != nil {
		t.Fatalf("create bare catalog: %v", err)
	}
	bareRelease := &model.ExtensionRelease{
		ExtensionID:     "test.bare",
		Version:         "1.0.0",
		ReleaseChannel:  "stable",
		MinCoreVersion:  "1.0.0",
		PublishedAtUnix: time.Now().Unix(),
		ManifestJSON:    model.JSON([]byte(`{"id":"test.bare","version":"1.0.0"}`)),
	}
	if err := db.WithContext(bg).Create(bareRelease).Error; err != nil {
		t.Fatalf("create bare release: %v", err)
	}

	s := NewService(svcCtx)

	install := func(extID, version string) uint {
		resp, err := s.Install(ctx, ExtensionInstallRequest{
			ExtensionID:    extID,
			ReleaseVersion: version,
			ScopeType:      "system",
			ScopeID:        "global",
			TargetType:     "agent",
			TargetID:       "default",
			Config:         map[string]any{"enabled": true},
		}, "test_admin")
		if err != nil {
			t.Fatalf("install %s: %v", extID, err)
		}
		return resp.InstallationID
	}

	analyticsID := install("test.analytics", "2.0.0")
	bareID := install("test.bare", "1.0.0")

	// Install-created instances always have Enabled=false (enable-manually-
	// after-install semantics): analytics gets enabled, bare stays disabled.
	if _, err := s.Enable(ctx, analyticsID, "test_admin"); err != nil {
		t.Fatalf("enable analytics: %v", err)
	}

	req := ExtensionInstallationListRequest{Page: 1, PageSize: 10}
	resp, err := s.InstallationList(ctx, req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byID := map[uint]ExtensionInstallationItem{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}

	// displayName joins the catalog display name.
	if got := byID[analyticsID].DisplayName; got != "Test Analytics" {
		t.Fatalf("expected display name joined from catalog, got: %q", got)
	}
	// A row without displayName -> falls back to name (no longer passes
	// extensionID through).
	if got := byID[bareID].DisplayName; got != "bare-name" {
		t.Fatalf("expected name fallback, got: %q", got)
	}
	// healthStatus derivation: enabled + not uninstalled -> healthy.
	if got := byID[analyticsID].HealthStatus; got != "healthy" {
		t.Fatalf("expected healthy, got: %q", got)
	}

	// A disabled instance immediately shows disabled in the list; the enabled
	// instance is unaffected.
	for _, item := range resp.Items {
		if item.ID == bareID && item.HealthStatus != "disabled" {
			t.Fatalf("expected disabled for disabled installation, got: %q", item.HealthStatus)
		}
		if item.ID == analyticsID && item.HealthStatus != "healthy" {
			t.Fatalf("analytics should be healthy, got: %q", item.HealthStatus)
		}
	}
}

// TestDeriveExtensionHealthStatus_Matrix: the derivation matrix (same
// semantics as HealthCheck).
