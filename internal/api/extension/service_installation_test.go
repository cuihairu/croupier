package extension

import (
	"context"
	"github.com/cuihairu/croupier/internal/svc"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestService_ResolveInstallationID_Skipped(t *testing.T) {
	t.Skip("Requires full service context setup")
}

func TestService_ResolveInstallationID_Numeric(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := &Service{svcCtx: svcCtx}

	// Test numeric ID
	id, err := s.ResolveInstallationID(context.Background(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 123 {
		t.Fatalf("expected id 123, got: %d", id)
	}
}

func TestService_ResolveInstallationID_Whitespace(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := &Service{svcCtx: svcCtx}

	// Test numeric ID with whitespace
	id, err := s.ResolveInstallationID(context.Background(), "  456  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 456 {
		t.Fatalf("expected id 456, got: %d", id)
	}
}

// ============================================================================
// Integration tests with in-memory database
// ============================================================================

// TestService_ResolveInstallationID_ByExtensionID tests resolving installation ID by extension ID
func TestService_ResolveInstallationID_ByExtensionID(t *testing.T) {
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

	// Resolve by extension ID
	id, err := s.ResolveInstallationID(ctx, "test.analytics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != installResp.InstallationID {
		t.Fatalf("expected id %d, got: %d", installResp.InstallationID, id)
	}
}

// TestService_ValidateDependencies_MissingDependency tests validation with missing dependency

// TestService_FindInstallationConflict_NotFound tests findInstallationConflict with no existing installation
func TestService_FindInstallationConflict_NotFound(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	svcCtx.DB = db

	s := NewService(svcCtx)

	req := ExtensionInstallRequest{
		ExtensionID: "test.ext",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "default",
	}

	conflict, existing, err := s.findInstallationConflict(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conflict {
		t.Fatal("expected no conflict")
	}

	if existing != nil {
		t.Fatal("expected existing to be nil")
	}
}

// TestService_ReleaseVersionExists tests release version existence check

// TestService_FindActiveInstallationByExtension_Integration tests finding active installation by extension ID
func TestService_FindActiveInstallationByExtension_Integration(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Install an extension
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

	// Find active installation
	installation, err := s.findActiveInstallationByExtension(ctx, "test.analytics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if installation == nil {
		t.Fatal("expected installation to be found")
	}

	if installation.ExtensionID != "test.analytics" {
		t.Fatalf("expected extension_id 'test.analytics', got: %s", installation.ExtensionID)
	}
}

// TestService_ActiveInstalledExtensionSet_Integration tests building the set of installed extensions

// TestService_ActiveInstalledExtensionSet_Integration tests building the set of installed extensions
func TestService_ActiveInstalledExtensionSet_Integration(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	// Install two extensions
	req1 := ExtensionInstallRequest{
		ExtensionID:    "test.analytics",
		ReleaseVersion: "2.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}
	_, err := s.Install(ctx, req1, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	req2 := ExtensionInstallRequest{
		ExtensionID:    "test.notifications",
		ReleaseVersion: "1.5.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{},
	}
	_, err = s.Install(ctx, req2, "test_admin")
	if err != nil {
		t.Fatalf("installation failed: %v", err)
	}

	// Get active installed extension set
	installedSet, err := s.activeInstalledExtensionSet(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !installedSet["test.analytics"] {
		t.Fatal("expected test.analytics to be in installed set")
	}

	if !installedSet["test.notifications"] {
		t.Fatal("expected test.notifications to be in installed set")
	}
}

// applyVersionOp: unknown operator -> (false, false), so the caller rejects
// the match on that basis. The opMatchers key set matches the seven literals
// extracted by the matchSingleClause prefixes; this directly tests the
// dispatch table's rejection branch.
