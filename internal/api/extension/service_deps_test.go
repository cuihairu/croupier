package extension

import (
	"github.com/cuihairu/croupier/internal/model"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestFormatDependentRef(t *testing.T) {
	item := model.ExtensionInstallation{ExtensionID: "official.analytics", ReleaseVersion: "1.2.3"}
	if got := formatDependentRef(item); got != "official.analytics@1.2.3" {
		t.Fatalf("unexpected dependent ref: %s", got)
	}
	item2 := model.ExtensionInstallation{ExtensionID: "official.analytics"}
	if got := formatDependentRef(item2); got != "official.analytics" {
		t.Fatalf("unexpected dependent ref without version: %s", got)
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

func TestParseDependencies_Complex(t *testing.T) {
	manifest := map[string]any{
		"dependencies": []any{
			"simple.extension",
			map[string]any{
				"id":       "complex.extension",
				"version":  "^1.2.0",
				"metadata": map[string]any{"optional": true},
			},
			map[string]any{
				"extension_id":     "another.ext",
				"required_version": ">=2.0.0",
			},
		},
	}
	deps := parseDependencies(manifest)
	if len(deps) != 3 {
		t.Fatalf("expected 3 dependencies, got: %d", len(deps))
	}
	if deps[1].Version != "^1.2.0" {
		t.Fatalf("unexpected version: %s", deps[1].Version)
	}
}

func TestValidateDependencies_EmptyManifest(t *testing.T) {
	// Test with nil manifest
	deps := parseDependencies(nil)
	if deps != nil {
		t.Fatal("expected nil for nil manifest")
	}

	// Test with empty manifest
	deps = parseDependencies(map[string]any{})
	if deps != nil {
		t.Fatal("expected nil for empty manifest")
	}
}

func TestValidateDependencies_NoDependenciesKey(t *testing.T) {
	manifest := map[string]any{
		"name":    "test",
		"version": "1.0.0",
	}
	deps := parseDependencies(manifest)
	if deps != nil {
		t.Fatal("expected nil when dependencies key is missing")
	}
}

func TestValidateDependencies_DependencyWithEmptyID(t *testing.T) {
	manifest := map[string]any{
		"dependencies": []any{
			"",
			"   ",
			map[string]any{},
		},
	}
	deps := parseDependencies(manifest)
	// Empty IDs should be skipped
	if len(deps) != 0 {
		t.Fatalf("expected 0 dependencies after filtering empty IDs, got: %d", len(deps))
	}
}

func TestDependencyTargetsExtension_CaseInsensitive(t *testing.T) {
	dep := extensionDependency{ExtensionID: "Test.Ext", Version: ">=1.0.0"}

	// Test case insensitive matching (normalizeExtensionID lowercases)
	// Since we're testing with different case, normalizeExtensionID makes them equal
	normalizedID := normalizeExtensionID(dep.ExtensionID)
	normalizedTarget := normalizeExtensionID("Test.Ext")
	if normalizedID != normalizedTarget {
		t.Fatalf("expected normalized IDs to match, got: %s vs %s", normalizedID, normalizedTarget)
	}
}

func TestDependencyTargetsExtension_EmptyVersion(t *testing.T) {
	// Empty version means any version matches
	dep := extensionDependency{ExtensionID: "test.ext", Version: ""}

	if !dependencyTargetsExtension(dep, "test.ext", "999.0.0") {
		t.Fatal("expected true for empty version constraint")
	}

	if !dependencyTargetsExtension(dep, "test.ext", "0.0.1") {
		t.Fatal("expected true for empty version constraint with low version")
	}
}

func TestDependencyTargetsExtension_DifferentExtension(t *testing.T) {
	dep := extensionDependency{ExtensionID: "test.ext", Version: ">=1.0.0"}

	if dependencyTargetsExtension(dep, "other.ext", "2.0.0") {
		t.Fatal("expected false for different extension ID")
	}
}

func TestFormatDependentRef_EmptyFields(t *testing.T) {
	item := model.ExtensionInstallation{
		ExtensionID:    "",
		ReleaseVersion: "",
	}
	ref := formatDependentRef(item)
	// formatDependentRef returns "unknown" for empty extension ID
	if ref != "unknown" {
		t.Fatalf("expected 'unknown' for empty extension ID, got: %s", ref)
	}

	item.ExtensionID = "test.ext"
	item.ReleaseVersion = ""
	ref = formatDependentRef(item)
	if ref != "test.ext" {
		t.Fatalf("expected extension ID only, got: %s", ref)
	}
}
