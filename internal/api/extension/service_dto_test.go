package extension

import (
	"context"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
	"time"
)

// Split from service_test.go; test bodies moved verbatim.

func TestToInstallationItem(t *testing.T) {
	modelItem := model.ExtensionInstallation{
		Model:           gorm.Model{ID: 123},
		InstallationKey: "test-key",
		ExtensionID:     "official.analytics",
		ReleaseVersion:  "1.2.3",
		ScopeType:       "system",
		ScopeID:         "global",
		TargetType:      "agent_group",
		TargetID:        "default",
		Status:          "enabled",
		DesiredState:    "enabled",
	}
	item := toInstallationItem(modelItem, "") // empty displayName -> falls back to extensionID
	if item.ID != 123 {
		t.Fatalf("unexpected id: %d", item.ID)
	}
	if item.InstallationKey != "test-key" {
		t.Fatalf("unexpected key: %s", item.InstallationKey)
	}
	if item.ExtensionID != "official.analytics" {
		t.Fatalf("unexpected extension_id: %s", item.ExtensionID)
	}
	if item.DisplayName != "official.analytics" {
		t.Fatalf("unexpected display_name: %s", item.DisplayName)
	}
	if item.Status != "enabled" {
		t.Fatalf("unexpected status: %s", item.Status)
	}
}

func TestToEventItems(t *testing.T) {
	modelEvents := []model.ExtensionEvent{
		{
			Model:       gorm.Model{ID: 1},
			EventType:   "install.started",
			Level:       "info",
			Message:     "Starting installation",
			PayloadJSON: model.JSON([]byte(`{"step":"download"}`)),
			CreatedBy:   "admin",
		},
		{
			Model:     gorm.Model{ID: 2},
			EventType: "install.completed",
			Level:     "info",
			Message:   "Installation complete",
			CreatedBy: "system",
		},
	}
	items := toEventItems(modelEvents)
	if len(items) != 2 {
		t.Fatalf("unexpected items length: %d", len(items))
	}
	if items[0].EventType != "install.started" {
		t.Fatalf("unexpected event_type: %s", items[0].EventType)
	}
	if items[0].Message != "Starting installation" {
		t.Fatalf("unexpected message: %s", items[0].Message)
	}
}

func TestToBindingItems(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "capability",
			BindingKey:  "analytics.query",
			TargetRef:   "agent1",
			Status:      "active",
			LastError:   "",
		},
		{
			BindingType: "function",
			BindingKey:  "external.onepanel.upgrade",
			TargetRef:   "agent2",
			Status:      "error",
			LastError:   "connection failed",
		},
	}
	items := toBindingItems(bindings)
	if len(items) != 2 {
		t.Fatalf("unexpected items length: %d", len(items))
	}
	if items[0].BindingType != "capability" {
		t.Fatalf("unexpected binding_type: %s", items[0].BindingType)
	}
	if items[0].BindingKey != "analytics.query" {
		t.Fatalf("unexpected binding_key: %s", items[0].BindingKey)
	}
	if items[1].LastError != "connection failed" {
		t.Fatalf("unexpected last_error: %s", items[1].LastError)
	}
}

func TestExtractCapabilities(t *testing.T) {
	manifest := map[string]any{
		"capabilities": []any{
			"analytics.query",
			map[string]any{"id": "ops.health"},
			map[string]any{"name": "notify.send"},
			"analytics.query", // duplicate
		},
	}
	got := extractCapabilities(manifest)
	if len(got) != 3 {
		t.Fatalf("unexpected capabilities count: %d", len(got))
	}
	if got[0] != "analytics.query" || got[1] != "ops.health" || got[2] != "notify.send" {
		t.Fatalf("unexpected capabilities: %+v", got)
	}
}

func TestExtractTags(t *testing.T) {
	manifest := map[string]any{
		"tags": []any{"ops", "analytics", "ops"}, // duplicate
	}
	tags := extractTags(manifest)
	if len(tags) != 2 {
		t.Fatalf("unexpected tags count: %d", len(tags))
	}
	if tags[0] != "ops" || tags[1] != "analytics" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
}

func TestExtractDefaultInstall(t *testing.T) {
	if !extractDefaultInstall(map[string]any{"default_install": true}) {
		t.Fatalf("expected true from bool")
	}
	if !extractDefaultInstall(map[string]any{"defaultInstall": "true"}) {
		t.Fatalf("expected true from string")
	}
	if extractDefaultInstall(map[string]any{"default_install": 0}) {
		t.Fatalf("expected false from zero")
	}
	if extractDefaultInstall(map[string]any{"default_install": false}) {
		t.Fatalf("expected false from bool false")
	}
}

func TestMapString(t *testing.T) {
	m := map[string]any{"key": "value"}
	if got := mapString(m, "key"); got != "value" {
		t.Fatalf("unexpected result, got=%s", got)
	}
	if got := mapString(m, "missing"); got != "" {
		t.Fatalf("expected empty string for missing key")
	}
	if got := mapString(nil, "key"); got != "" {
		t.Fatalf("expected empty string for nil map")
	}
}

func TestResolveConfigSchemaEmptyExtensionID(t *testing.T) {
	db, _ := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	svcCtx := &svc.ServiceContext{DB: db}
	s := &Service{svcCtx: svcCtx}

	schema := s.resolveConfigSchema(context.Background(), "", "1.0.0")
	if len(schema) != 0 {
		t.Fatalf("expected empty schema for empty extension id, got: %+v", schema)
	}
}

func TestResolveConfigSchema(t *testing.T) {
	// Test with empty extension ID returns empty schema
	db, _ := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	svcCtx := &svc.ServiceContext{DB: db}
	s := &Service{svcCtx: svcCtx}

	schema := s.resolveConfigSchema(context.Background(), "", "1.0.0")
	if len(schema) != 0 {
		t.Fatalf("expected empty schema for empty extension id, got: %+v", schema)
	}
}

func TestToInstallationItem_WithTimestamp(t *testing.T) {
	modelItem := model.ExtensionInstallation{
		Model:          gorm.Model{ID: 123, UpdatedAt: time.Now()},
		ExtensionID:    "test.ext",
		ReleaseVersion: "1.0.0",
		Status:         "enabled",
		DesiredState:   "enabled",
		Enabled:        true,
	}
	item := toInstallationItem(modelItem, "")
	if item.ID != 123 {
		t.Fatalf("unexpected id: %d", item.ID)
	}
	if item.UpdatedAt == 0 {
		t.Fatal("expected non-zero UpdatedAt")
	}
}

func TestResolveConfigSchema_EmptyVersion(t *testing.T) {
	// This test requires a full ServiceContext setup with Extensions.Catalog
	// For now, just test that the function doesn't panic with nil context
	// The actual schema resolution is tested in integration tests
	t.Skip("Requires full service context setup")
}

func TestExtractCapabilities_NestedMap(t *testing.T) {
	manifest := map[string]any{
		"capabilities": []any{
			map[string]any{
				"id":       "analytics.query",
				"metadata": map[string]any{"priority": 1},
			},
			map[string]any{
				"capability": "notify.send",
			},
		},
	}
	got := extractCapabilities(manifest)
	// extractCapabilities looks for "id" or "name" in nested maps
	// The second map has "capability" which isn't recognized
	if len(got) != 1 {
		t.Fatalf("expected 1 capability (only 'id' is recognized), got: %d: %v", len(got), got)
	}
	if got[0] != "analytics.query" {
		t.Fatalf("expected analytics.query, got: %s", got[0])
	}
}

func TestExtractTags_MixedTypes(t *testing.T) {
	manifest := map[string]any{
		"tags": []any{"ops", 123, true, "analytics"},
	}
	tags := extractTags(manifest)
	if len(tags) != 4 {
		t.Fatalf("expected 4 tags, got: %d", len(tags))
	}
}

func TestMapString_Conversions(t *testing.T) {
	m := map[string]any{
		"string": "value",
		"number": 123,
		"bool":   true,
		"float":  3.14,
		"nil":    nil,
		"empty":  "",
		"<nil>":  "<nil>",
	}
	cases := []struct {
		key      string
		expected string
	}{
		{"string", "value"},
		{"number", "123"},
		{"bool", "true"},
		{"float", "3.14"},
		{"nil", ""},
		{"empty", ""},
		{"<nil>", ""},
		{"missing", ""},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			got := mapString(m, c.key)
			if got != c.expected {
				t.Fatalf("unexpected value for %s, got=%s expect=%s", c.key, got, c.expected)
			}
		})
	}
}

func TestMapString_NilMap(t *testing.T) {
	got := mapString(nil, "key")
	if got != "" {
		t.Fatalf("expected empty string for nil map, got: %s", got)
	}
}

func TestExtractCapabilities_NoCapabilities(t *testing.T) {
	// Test with manifest without capabilities
	caps := extractCapabilities(map[string]any{})
	if len(caps) != 0 {
		t.Fatalf("expected empty capabilities for empty manifest")
	}

	caps = extractCapabilities(map[string]any{"other": "data"})
	if len(caps) != 0 {
		t.Fatalf("expected empty capabilities when capabilities key is missing")
	}
}

func TestExtractCapabilities_NotAnArray(t *testing.T) {
	manifest := map[string]any{
		"capabilities": "not-an-array",
	}
	caps := extractCapabilities(manifest)
	if len(caps) != 0 {
		t.Fatalf("expected empty capabilities for non-array value")
	}
}

func TestExtractTags_NotAnArray(t *testing.T) {
	manifest := map[string]any{
		"tags": "not-an-array",
	}
	tags := extractTags(manifest)
	if len(tags) != 0 {
		t.Fatalf("expected empty tags for non-array value")
	}
}

func TestExtractDefaultInstall_NilManifest(t *testing.T) {
	result := extractDefaultInstall(nil)
	if result {
		t.Fatal("expected false for nil manifest")
	}
}

func TestExtractDefaultInstall_UnknownType(t *testing.T) {
	manifest := map[string]any{
		"default_install": []string{"yes"},
	}
	result := extractDefaultInstall(manifest)
	if result {
		t.Fatal("expected false for unknown type")
	}
}

func TestToInstallationItem_AllFields(t *testing.T) {
	now := time.Now()
	modelItem := model.ExtensionInstallation{
		Model:           gorm.Model{ID: 999, UpdatedAt: now},
		InstallationKey: "test-key",
		ExtensionID:     "official.analytics",
		ReleaseVersion:  "2.5.0",
		ScopeType:       "game",
		ScopeID:         "demo",
		TargetType:      "agent_group",
		TargetID:        "default",
		Status:          "enabled",
		DesiredState:    "enabled",
		Enabled:         true,
		LastError:       "some error",
	}

	item := toInstallationItem(modelItem, "")
	assert.Equal(t, uint(999), item.ID)
	assert.Equal(t, "test-key", item.InstallationKey)
	assert.Equal(t, "official.analytics", item.ExtensionID)
	assert.Equal(t, "official.analytics", item.DisplayName)
	assert.Equal(t, "2.5.0", item.ReleaseVersion)
	assert.Equal(t, "game", item.ScopeType)
	assert.Equal(t, "demo", item.ScopeID)
	assert.Equal(t, "agent_group", item.TargetType)
	assert.Equal(t, "default", item.TargetID)
	assert.Equal(t, "enabled", item.Status)
	assert.Equal(t, "enabled", item.DesiredState)
	assert.True(t, item.Enabled)
	// #46 batch 2: healthStatus is derived from status/enabled (enabled +
	// not uninstalled -> healthy), no longer always unknown.
	assert.Equal(t, "healthy", item.HealthStatus)
	assert.Equal(t, "some error", item.LastError)
	assert.Equal(t, now.Unix(), item.UpdatedAt)
}

func TestToEventItems_Empty(t *testing.T) {
	items := toEventItems([]model.ExtensionEvent{})
	if len(items) != 0 {
		t.Fatalf("expected empty items")
	}
}

func TestToBindingItems_Empty(t *testing.T) {
	items := toBindingItems([]model.ExtensionRuntimeBinding{})
	if len(items) != 0 {
		t.Fatalf("expected empty items")
	}
}
