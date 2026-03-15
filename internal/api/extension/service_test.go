package extension

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidateConfigAgainstSchemaRequired(t *testing.T) {
	schema := map[string]any{
		"required": []any{"enabled"},
		"properties": map[string]any{
			"enabled": map[string]any{"type": "boolean"},
		},
	}

	err := validateConfigAgainstSchema(map[string]any{}, schema)
	if err == nil {
		t.Fatalf("expected missing required error")
	}
}

func TestValidateConfigAgainstSchemaTypeAndEnum(t *testing.T) {
	schema := map[string]any{
		"required": []any{"mode", "retry"},
		"properties": map[string]any{
			"mode":  map[string]any{"type": "string", "enum": []any{"safe", "fast"}},
			"retry": map[string]any{"type": "integer"},
		},
	}

	valid := map[string]any{
		"mode":  "safe",
		"retry": float64(3),
	}
	if err := validateConfigAgainstSchema(valid, schema); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	invalidEnum := map[string]any{
		"mode":  "invalid",
		"retry": float64(3),
	}
	if err := validateConfigAgainstSchema(invalidEnum, schema); err == nil {
		t.Fatalf("expected enum validation error")
	}

	invalidType := map[string]any{
		"mode":  "safe",
		"retry": "3",
	}
	if err := validateConfigAgainstSchema(invalidType, schema); err == nil {
		t.Fatalf("expected integer type validation error")
	}
}

func TestIsJSONIntegerType(t *testing.T) {
	cases := []struct {
		name   string
		value  any
		expect bool
	}{
		{name: "int", value: int(1), expect: true},
		{name: "float_integer", value: float64(2), expect: true},
		{name: "float_fraction", value: float64(2.5), expect: false},
		{name: "string", value: "2", expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isJSONIntegerType(c.value)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestParseDependencies(t *testing.T) {
	manifest := map[string]any{
		"dependencies": []any{
			"official.base",
			map[string]any{"id": "official.analytics"},
			map[string]any{"extension_id": "official.notify", "required_version": "1.2.0"},
			map[string]any{"id": "official.ops", "version": "2.0.0"},
		},
	}
	deps := parseDependencies(manifest)
	if len(deps) != 4 {
		t.Fatalf("unexpected dependencies length: %d", len(deps))
	}
	if deps[0].ExtensionID != "official.base" {
		t.Fatalf("unexpected first dep id: %s", deps[0].ExtensionID)
	}
	if deps[2].ExtensionID != "official.notify" || deps[2].Version != "1.2.0" {
		t.Fatalf("unexpected third dep: %+v", deps[2])
	}
	if deps[3].ExtensionID != "official.ops" || deps[3].Version != "2.0.0" {
		t.Fatalf("unexpected fourth dep: %+v", deps[3])
	}
}

func TestMatchVersionConstraint(t *testing.T) {
	cases := []struct {
		name       string
		current    string
		constraint string
		expect     bool
	}{
		{name: "exact_true", current: "1.2.3", constraint: "1.2.3", expect: true},
		{name: "exact_false", current: "1.2.4", constraint: "1.2.3", expect: false},
		{name: "gte_true", current: "1.3.0", constraint: ">=1.2.3", expect: true},
		{name: "lt_true", current: "1.2.2", constraint: "<1.2.3", expect: true},
		{name: "range_true", current: "1.4.0", constraint: ">=1.2.0, <2.0.0", expect: true},
		{name: "range_false", current: "2.1.0", constraint: ">=1.2.0, <2.0.0", expect: false},
		{name: "caret_true", current: "1.5.1", constraint: "^1.2.3", expect: true},
		{name: "caret_false", current: "2.0.0", constraint: "^1.2.3", expect: false},
		{name: "tilde_true", current: "1.2.9", constraint: "~1.2.3", expect: true},
		{name: "tilde_false", current: "1.3.0", constraint: "~1.2.3", expect: false},
		{name: "v_prefix", current: "v1.2.3", constraint: ">=1.2.0", expect: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchVersionConstraint(c.current, c.constraint)
			if got != c.expect {
				t.Fatalf("unexpected result, current=%s constraint=%s got=%v expect=%v", c.current, c.constraint, got, c.expect)
			}
		})
	}
}

func TestDependencyTargetsExtension(t *testing.T) {
	dep := extensionDependency{ExtensionID: "official.analytics", Version: ">=1.2.0, <2.0.0"}
	if !dependencyTargetsExtension(dep, "official.analytics", "1.5.0") {
		t.Fatalf("expected dependency to match target extension version")
	}
	if dependencyTargetsExtension(dep, "official.analytics", "2.1.0") {
		t.Fatalf("expected dependency version mismatch")
	}
	if dependencyTargetsExtension(dep, "official.notify", "1.5.0") {
		t.Fatalf("expected different extension id not to match")
	}
}

func TestIsActiveInstallation(t *testing.T) {
	active := model.ExtensionInstallation{Status: "enabled", DesiredState: "enabled"}
	if !isActiveInstallation(active) {
		t.Fatalf("expected enabled installation to be active")
	}
	inactive := model.ExtensionInstallation{Status: "uninstalled", DesiredState: "uninstalled"}
	if isActiveInstallation(inactive) {
		t.Fatalf("expected uninstalled installation to be inactive")
	}
}

func TestExtractCapabilities(t *testing.T) {
	manifest := map[string]any{
		"capabilities": []any{
			"analytics.query",
			map[string]any{"id": "ops.health"},
			map[string]any{"name": "notify.send"},
			"analytics.query",
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
		"tags": []any{"ops", "analytics", "ops"},
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
}

func TestFormatDependentRef(t *testing.T) {
	row := model.ExtensionInstallation{ExtensionID: "official.analytics", ReleaseVersion: "1.2.3"}
	if got := formatDependentRef(row); got != "official.analytics@1.2.3" {
		t.Fatalf("unexpected dependent ref: %s", got)
	}
	row2 := model.ExtensionInstallation{ExtensionID: "official.analytics"}
	if got := formatDependentRef(row2); got != "official.analytics" {
		t.Fatalf("unexpected dependent ref without version: %s", got)
	}
}

func TestFindInstallationConflictReturnsActive(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	seed := []model.ExtensionInstallation{
		{
			InstallationKey: "k1",
			ExtensionID:     "official.analytics",
			ReleaseVersion:  "1.0.0",
			ScopeType:       "system",
			ScopeID:         "global",
			TargetType:      "agent_group",
			TargetID:        "default",
			Status:          "uninstalled",
			DesiredState:    "uninstalled",
			InstalledBy:     "tester",
		},
		{
			InstallationKey: "k2",
			ExtensionID:     "official.analytics",
			ReleaseVersion:  "1.1.0",
			ScopeType:       "system",
			ScopeID:         "global",
			TargetType:      "agent_group",
			TargetID:        "default",
			Status:          "enabled",
			DesiredState:    "enabled",
			InstalledBy:     "tester",
		},
	}
	for _, row := range seed {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed row failed: %v", err)
		}
	}

	s := &Service{svcCtx: &svc.ServiceContext{DB: db}}
	conflict, existing, err := s.findInstallationConflict(context.Background(), ExtensionInstallRequest{
		ExtensionID: "official.analytics",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent_group",
		TargetID:    "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conflict {
		t.Fatalf("expected conflict=true")
	}
	if existing == nil {
		t.Fatalf("expected existing installation")
	}
	if existing.InstallationKey != "k2" {
		t.Fatalf("expected active installation k2, got %s", existing.InstallationKey)
	}
}

func TestFindInstallationConflictIgnoresUninstalled(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ExtensionInstallation{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	row := model.ExtensionInstallation{
		InstallationKey: "k1",
		ExtensionID:     "official.analytics",
		ReleaseVersion:  "1.0.0",
		ScopeType:       "system",
		ScopeID:         "global",
		TargetType:      "agent_group",
		TargetID:        "default",
		Status:          "uninstalled",
		DesiredState:    "uninstalled",
		InstalledBy:     "tester",
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row failed: %v", err)
	}

	s := &Service{svcCtx: &svc.ServiceContext{DB: db}}
	conflict, existing, err := s.findInstallationConflict(context.Background(), ExtensionInstallRequest{
		ExtensionID: "official.analytics",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent_group",
		TargetID:    "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflict {
		t.Fatalf("expected conflict=false when only uninstalled rows exist")
	}
	if existing != nil {
		t.Fatalf("expected existing=nil when no active conflict")
	}
}

func TestExtractCapabilityDetailsFromBindings(t *testing.T) {
	caps, details := extractCapabilityDetailsFromBindings([]model.ExtensionRuntimeBinding{
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			SpecJSON:    `{"provider":"onepanel","operations":["list_apps","install_app"]}`,
		},
		{
			BindingType: "function",
			BindingKey:  "external.onepanel.upgrade_app",
		},
	})
	if len(caps) < 3 {
		t.Fatalf("expected capabilities with raw binding + external capability, got: %+v", caps)
	}
	hasExternalCapability := false
	for _, cap := range caps {
		if cap == "external.onepanel" {
			hasExternalCapability = true
			break
		}
	}
	if !hasExternalCapability {
		t.Fatalf("expected external.onepanel in capabilities: %+v", caps)
	}

	if len(details) == 0 {
		t.Fatalf("expected structured capability details")
	}
	var onepanel *ExtensionCapabilityDetail
	for i := range details {
		if details[i].Capability == "external.onepanel" {
			onepanel = &details[i]
			break
		}
	}
	if onepanel == nil {
		t.Fatalf("expected external.onepanel detail, got: %+v", details)
	}
	if len(onepanel.Operations) != 3 {
		t.Fatalf("expected merged operations [list_apps install_app upgrade_app], got: %+v", onepanel.Operations)
	}
}
