package extension

import (
	"context"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

// TestService_UpdateConfig_Success tests successful config update
func TestService_UpdateConfig_Success(t *testing.T) {
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

	// Update config
	updateReq := ExtensionConfigUpdateRequest{
		Config: map[string]any{"enabled": false, "interval": 120},
	}

	resp, err := s.UpdateConfig(ctx, installResp.InstallationID, updateReq, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "updated" {
		t.Fatalf("expected status 'updated', got: %s", resp.Status)
	}
}

// TestService_ConfigSchema_Success tests config schema retrieval

// TestService_ConfigSchema_Success tests config schema retrieval
func TestService_ConfigSchema_Success(t *testing.T) {
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

	// Get config schema
	resp, err := s.ConfigSchema(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Schema) == 0 {
		t.Fatal("expected non-empty config schema from manifest")
	}
}

// TestService_Config_Success tests config retrieval

// TestService_Config_Success tests config retrieval
func TestService_Config_Success(t *testing.T) {
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
		SecretRefs:     map[string]string{"api_key": "secret:key1"},
	}
	installResp, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Get config
	resp, err := s.Config(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Config == nil {
		t.Fatal("expected config to be non-nil")
	}

	if len(resp.SecretRefs) == 0 {
		t.Fatal("expected secret_refs to be non-empty")
	}
}

// TestService_TestConnection_Success tests test connection

// TestService_TestConnection_Success tests test connection
func TestService_TestConnection_Success(t *testing.T) {
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

	// Test connection
	resp, err := s.TestConnection(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "disabled" { // New installations start disabled
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}
}

// TestService_Capabilities_Success tests capabilities retrieval

// TestService_Capabilities_Success tests capabilities retrieval
func TestService_Capabilities_Success(t *testing.T) {
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

	// Get capabilities
	resp, err := s.Capabilities(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have capabilities from manifest since bindings are empty
	if len(resp.Capabilities) == 0 {
		t.Fatal("expected capabilities from manifest")
	}
}

// TestService_Pages_Success tests pages retrieval

// TestService_Pages_Success tests pages retrieval
func TestService_Pages_Success(t *testing.T) {
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

	// Get pages
	resp, err := s.Pages(ctx, installResp.InstallationID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pages list should be returned (empty if not defined)
	if resp.Pages == nil {
		t.Fatal("expected pages list to be initialized")
	}
}

// TestService_HealthCheck_Success tests health check

// TestService_HealthCheck_Success tests health check
func TestService_HealthCheck_Success(t *testing.T) {
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

	// Health check
	resp, err := s.HealthCheck(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "disabled" { // New installations start disabled
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}

	if resp.CheckedAt == 0 {
		t.Fatal("expected checked_at to be set")
	}
}

// TestService_Enable_Success tests enable operation

// TestService_Events_Success tests events retrieval
func TestService_Events_Success(t *testing.T) {
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

	// Get events
	eventReq := ExtensionEventListRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.Events(ctx, installResp.InstallationID, eventReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Events list should be returned (may be empty initially)
	if resp.Items == nil {
		t.Fatal("expected events list to be initialized")
	}
}

// TestService_AgentSyncPayload_Success tests agent sync payload retrieval

// TestService_AgentSyncPayload_Success tests agent sync payload retrieval
func TestService_AgentSyncPayload_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create an installation targeted at all agents
	req := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	_, err := s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Get agent sync payload
	resp, err := s.AgentSyncPayload(ctx, "test-agent-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Payload == nil {
		t.Fatal("expected payload to be non-nil")
	}

	// The Payload field contains the actual AgentSyncPayload
	payload, ok := resp.Payload.(*extensionsync.AgentSyncPayload)
	if !ok {
		t.Fatalf("expected payload to be *AgentSyncPayload, got: %T", resp.Payload)
	}

	if payload.AgentID != "test-agent-001" {
		t.Fatalf("expected agent_id 'test-agent-001', got: %s", payload.AgentID)
	}
}

// TestService_ResolveInstallationID_ByExtensionID tests resolving installation ID by extension ID

// TestService_ValidateDependencies_MissingDependency tests validation with missing dependency
func TestService_ValidateDependencies_MissingDependency(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Try to install test.dependent without test.analytics installed
	req := ExtensionInstallRequest{
		ExtensionID:    "test.dependent",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{},
	}

	_, err := s.Install(ctx, req, "test_admin")
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

// TestService_ValidateDependencies_WithInstalledDependency tests validation with installed dependency

// TestService_ValidateDependencies_WithInstalledDependency tests validation with installed dependency
func TestService_ValidateDependencies_WithInstalledDependency(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// First install the dependency
	depReq := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	_, err := s.Install(ctx, depReq, "test_admin")
	if err != nil {
		t.Fatalf("failed to install dependency: %v", err)
	}

	// Now install the dependent extension
	req := ExtensionInstallRequest{
		ExtensionID:    "test.dependent",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{},
	}

	_, err = s.Install(ctx, req, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error with dependency installed: %v", err)
	}
}

// TestService_HealthCheck_UninstalledStatus tests health check for uninstalled extension

// TestService_HealthCheck_UninstalledStatus tests health check for uninstalled extension
func TestService_HealthCheck_UninstalledStatus(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Create and then uninstall an installation
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

	_, err = s.Uninstall(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	// Health check should return uninstalled status
	resp, err := s.HealthCheck(ctx, installResp.InstallationID, "test_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "uninstalled" {
		t.Fatalf("expected status 'uninstalled', got: %s", resp.Status)
	}
}

// TestService_FindInstallationConflict_NotFound tests findInstallationConflict with no existing installation

// TestService_ReleaseVersionExists tests release version existence check
func TestService_ReleaseVersionExists(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Test existing version
	exists, err := s.releaseVersionExists(context.Background(), "test.analytics", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected version to exist")
	}

	// Test non-existing version
	exists, err = s.releaseVersionExists(context.Background(), "test.analytics", "99.99.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected version to not exist")
	}
}

// TestService_ResolveManifestForRelease tests manifest resolution for a release

// TestService_ResolveManifestForRelease tests manifest resolution for a release
func TestService_ResolveManifestForRelease(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	manifest, err := s.resolveManifestForRelease(context.Background(), "test.analytics", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manifest == nil {
		t.Fatal("expected manifest to be non-nil")
	}

	if id, ok := manifest["id"].(string); !ok || id != "test.analytics" {
		t.Fatalf("expected manifest id 'test.analytics', got: %v", manifest["id"])
	}
}

// TestService_ResolveManifestForRelease_NotFound tests manifest resolution for non-existent release

// TestService_ResolveManifestForRelease_NotFound tests manifest resolution for non-existent release
func TestService_ResolveManifestForRelease_NotFound(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	manifest, err := s.resolveManifestForRelease(context.Background(), "test.analytics", "99.99.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manifest == nil {
		t.Fatal("expected manifest to be non-nil (empty map)")
	}

	if len(manifest) != 0 {
		t.Fatal("expected empty manifest for non-existent release")
	}
}

// TestService_EnsureNoActiveDependents tests dependency blocking on uninstall

// TestService_EnsureNoActiveDependents tests dependency blocking on uninstall
func TestService_EnsureNoActiveDependents(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Install dependency first
	depReq := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	depResp, err := s.Install(ctx, depReq, "test_admin")
	if err != nil {
		t.Fatalf("failed to install dependency: %v", err)
	}

	// Install dependent extension
	dependentReq := ExtensionInstallRequest{
		ExtensionID:    "test.dependent",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{},
	}
	_, err = s.Install(ctx, dependentReq, "test_admin")
	if err != nil {
		t.Fatalf("failed to install dependent: %v", err)
	}

	// Try to uninstall the dependency - should fail due to active dependent
	err = s.ensureNoActiveDependents(ctx, depResp.InstallationID)
	if err == nil {
		t.Fatal("expected error when uninstalling extension with active dependents")
	}
}

// TestService_FindActiveInstallationByExtension_Integration tests finding active installation by extension ID
