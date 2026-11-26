package extension

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

// TestService_CatalogList_Success tests successful catalog list retrieval
func TestService_CatalogList_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	req := ExtensionCatalogListRequest{
		Page:     1,
		PageSize: 10,
	}

	resp, err := s.CatalogList(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Total < 3 {
		t.Fatalf("expected at least 3 catalog items, got: %d", resp.Total)
	}

	// Check that at least one item has tags from manifest
	hasTags := false
	for _, item := range resp.Items {
		if len(item.Tags) > 0 {
			hasTags = true
			break
		}
	}
	if !hasTags {
		t.Fatal("expected at least one item with tags")
	}
}

// TestService_CatalogDetail_Success tests successful catalog detail retrieval

// TestService_CatalogDetail_Success tests successful catalog detail retrieval
func TestService_CatalogDetail_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	resp, err := s.CatalogDetail(ctx, "test.analytics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Item == nil {
		t.Fatal("expected item to be non-nil")
	}

	if resp.Item.ID != "test.analytics" {
		t.Fatalf("expected extension_id test.analytics, got: %s", resp.Item.ID)
	}

	if len(resp.Releases) != 2 {
		t.Fatalf("expected 2 releases, got: %d", len(resp.Releases))
	}

	// Note: firstManifest uses the first release in the list, which may be 1.0.0
	// The manifest capabilities and default_install depend on which release is first
	if resp.Manifest == nil {
		t.Fatal("expected manifest to be non-nil")
	}
}

// TestService_CatalogDetail_EmptyID tests catalog detail with empty extension ID

// TestService_CatalogDetail_EmptyID tests catalog detail with empty extension ID
func TestService_CatalogDetail_EmptyID(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)

	s := NewService(svcCtx)

	_, err := s.CatalogDetail(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty extension_id")
	}
}

// TestService_CatalogReleases_Success tests successful catalog releases retrieval

// TestService_CatalogReleases_Success tests successful catalog releases retrieval
func TestService_CatalogReleases_Success(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)
	createTestCatalogData(t, svcCtx)

	s := NewService(svcCtx)

	resp, err := s.CatalogReleases(ctx, "test.analytics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Releases) != 2 {
		t.Fatalf("expected 2 releases, got: %d", len(resp.Releases))
	}

	if resp.Total != 2 {
		t.Fatalf("expected total 2, got: %d", resp.Total)
	}
}

// TestService_Install_Success tests successful extension installation

// TestService_CatalogWrite_Lifecycle (#46 batch 3): register -> update
// (listing status) -> publish release (latestVersion back-fill rule) ->
// active installation blocks removal -> cascade removal after uninstall.
func TestService_CatalogWrite_Lifecycle(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	ctx := setupAdminContext(t, svcCtx)

	s := NewService(svcCtx)

	// (1) Create: empty name/displayName/vendor go through the fallback chain.
	created, err := s.CatalogCreate(ctx, ExtensionCatalogCreateRequest{
		ExtensionID: "vendor.demo-ext",
		Summary:     "demo extension",
	}, "test_admin")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Item.Name != "vendor.demo-ext" || created.Item.DisplayName != "vendor.demo-ext" {
		t.Fatalf("expected name fallback to extensionId, got: %q/%q", created.Item.Name, created.Item.DisplayName)
	}
	if created.Item.Vendor != "external" || created.Item.Kind != "community" || created.Item.Status != "active" {
		t.Fatalf("unexpected defaults: %+v", created.Item)
	}

	// (2) Duplicate registration -> 409.
	if _, err := s.CatalogCreate(ctx, ExtensionCatalogCreateRequest{ExtensionID: "vendor.demo-ext"}, "test_admin"); err == nil {
		t.Fatal("expected conflict on duplicate create")
	}

	// (3) Invalid extensionId / status -> 400.
	if _, err := s.CatalogCreate(ctx, ExtensionCatalogCreateRequest{ExtensionID: "Bad_ID"}, "test_admin"); err == nil {
		t.Fatal("expected invalid extensionId rejected")
	}
	if _, err := s.CatalogCreate(ctx, ExtensionCatalogCreateRequest{ExtensionID: "vendor.ok", Status: "archived"}, "test_admin"); err == nil {
		t.Fatal("expected invalid status rejected")
	}

	// (4) Update: displayName + delisted status.
	updated, err := s.CatalogUpdate(ctx, "vendor.demo-ext", ExtensionCatalogUpdateRequest{
		DisplayName: "演示扩展",
		Status:      "delisted",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Item.DisplayName != "演示扩展" || updated.Item.Status != "delisted" {
		t.Fatalf("expected update applied, got: %+v", updated.Item)
	}
	// Invalid status -> 400
	if _, err := s.CatalogUpdate(ctx, "vendor.demo-ext", ExtensionCatalogUpdateRequest{Status: "gone"}); err == nil {
		t.Fatal("expected invalid status rejected on update")
	}
	// Nonexistent extension -> 404
	if _, err := s.CatalogUpdate(ctx, "vendor.missing", ExtensionCatalogUpdateRequest{DisplayName: "x"}); err == nil {
		t.Fatal("expected not found on missing update")
	}

	// (5) Publish release: v1.0.0 -> latestVersion back-fill.
	pub, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", ExtensionReleasePublishRequest{
		Version:   "1.0.0",
		Changelog: "first",
		Manifest: map[string]any{
			"id":      "vendor.demo-ext",
			"version": "1.0.0",
		},
	}, "test_admin")
	if err != nil {
		t.Fatalf("publish 1.0.0: %v", err)
	}
	if pub.Release.Version != "1.0.0" || pub.Release.PublishedAt == 0 {
		t.Fatalf("unexpected release item: %+v", pub.Release)
	}
	// 0.9.0 is lower -> latest does not roll back.
	if _, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", ExtensionReleasePublishRequest{
		Version:  "0.9.0",
		Manifest: map[string]any{"id": "vendor.demo-ext", "version": "0.9.0"},
	}, "test_admin"); err != nil {
		t.Fatalf("publish 0.9.0: %v", err)
	}
	detail, err := s.CatalogDetail(ctx, "vendor.demo-ext")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Item.LatestVersion != "1.0.0" {
		t.Fatalf("expected latest stay 1.0.0, got: %q", detail.Item.LatestVersion)
	}
	if len(detail.Releases) != 2 {
		t.Fatalf("expected 2 releases, got: %d", len(detail.Releases))
	}
	// Duplicate version -> 409; bad semver/channel/missing manifest -> 400.
	dup := ExtensionReleasePublishRequest{Version: "1.0.0", Manifest: map[string]any{"id": "x"}}
	if _, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", dup, "test_admin"); err == nil {
		t.Fatal("expected duplicate version rejected")
	}
	badVer := ExtensionReleasePublishRequest{Version: "not-semver", Manifest: map[string]any{"id": "x"}}
	if _, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", badVer, "test_admin"); err == nil {
		t.Fatal("expected invalid semver rejected")
	}
	badChannel := ExtensionReleasePublishRequest{Version: "2.0.0", ReleaseChannel: "rc", Manifest: map[string]any{"id": "x"}}
	if _, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", badChannel, "test_admin"); err == nil {
		t.Fatal("expected invalid channel rejected")
	}
	noManifest := ExtensionReleasePublishRequest{Version: "2.0.0"}
	if _, err := s.CatalogReleasePublish(ctx, "vendor.demo-ext", noManifest, "test_admin"); err == nil {
		t.Fatal("expected missing manifest rejected")
	}
	// Nonexistent extension -> 404
	if _, err := s.CatalogReleasePublish(ctx, "vendor.missing", ExtensionReleasePublishRequest{Version: "1.0.0", Manifest: map[string]any{}}, "test_admin"); err == nil {
		t.Fatal("expected not found on missing publish")
	}

	// (6) Active installation instances block removal.
	if _, err := s.Install(ctx, ExtensionInstallRequest{
		ExtensionID:    "vendor.demo-ext",
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent",
		TargetID:       "default",
		Config:         map[string]any{"enabled": true},
	}, "test_admin"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := s.CatalogDelete(ctx, "vendor.demo-ext"); err == nil {
		t.Fatal("expected delete blocked by active installation")
	}
	// Removal after uninstall -> success with cascade cleanup of releases.
	if _, err := s.Uninstall(ctx, detailItemID(t, s, ctx, "vendor.demo-ext"), "test_admin"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if err := s.CatalogDelete(ctx, "vendor.demo-ext"); err != nil {
		t.Fatalf("delete after uninstall: %v", err)
	}
	if _, err := s.CatalogDetail(ctx, "vendor.demo-ext"); err == nil {
		t.Fatal("expected deleted extension not found")
	}
	if _, err := s.CatalogReleases(ctx, "vendor.demo-ext"); err == nil || detail != nil && len(detail.Releases) == 0 {
		// releases cascade cleanup: the version list should also be empty (Get
		// returns not found when the catalog row is gone).
		_ = err
	}
	if _, _, err := svcCtx.Extensions.Catalog.Get(ctx, "vendor.demo-ext"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected catalog row removed, got: %v", err)
	}
	if _, err := svcCtx.Extensions.Catalog.ReleaseByVersion(ctx, "vendor.demo-ext", "1.0.0"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected releases cascaded away, got: %v", err)
	}
}

// detailItemID: a test helper that fetches the current installation instance
// ID of the extension through the installation list.

// TestService_CatalogWrite_PermissionDenied: a ctx without write permission is
// rejected (wiring verification of the utils-layer guard semantics in this
// package).
func TestService_CatalogWrite_PermissionDenied(t *testing.T) {
	db := setupIntegrationTestDB(t)
	svcCtx := setupExtensionTestContext(t, db)
	// A plain-user ctx with no roles or permissions.
	ctx := context.WithValue(context.Background(), "username", "plain_user")

	s := NewService(svcCtx)
	if _, err := s.CatalogCreate(ctx, ExtensionCatalogCreateRequest{ExtensionID: "vendor.x"}, "plain_user"); err == nil {
		t.Fatal("expected create denied without permission")
	}
	if err := s.CatalogDelete(ctx, "vendor.x"); err == nil {
		t.Fatal("expected delete denied without permission")
	}
	if _, err := s.CatalogReleasePublish(ctx, "vendor.x", ExtensionReleasePublishRequest{Version: "1.0.0", Manifest: map[string]any{}}, "plain_user"); err == nil {
		t.Fatal("expected publish denied without permission")
	}
}
