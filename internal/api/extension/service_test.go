package extension

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMapServiceError(t *testing.T) {
	if mapServiceError(nil) != nil {
		t.Fatalf("expected nil for nil error")
	}
	err := mapServiceError(gorm.ErrRecordNotFound)
	if err == nil {
		t.Fatalf("expected error for record not found")
	}

	err = mapServiceError(gorm.ErrInvalidDB)
	if err == nil {
		t.Fatalf("expected error for invalid db")
	}

	err = mapServiceError(gorm.ErrDuplicatedKey)
	if err == nil {
		t.Fatalf("expected error for duplicated key")
	}

	customErr := gorm.ErrInvalidTransaction
	if mapServiceError(customErr) != customErr {
		t.Fatalf("expected custom error to pass through")
	}
}

func TestIsJSONNumberType(t *testing.T) {
	cases := []struct {
		name  string
		value any
		expect bool
	}{
		{name: "int", value: int(1), expect: true},
		{name: "int8", value: int8(1), expect: true},
		{name: "int16", value: int16(1), expect: true},
		{name: "int32", value: int32(1), expect: true},
		{name: "int64", value: int64(1), expect: true},
		{name: "uint", value: uint(1), expect: true},
		{name: "uint8", value: uint8(1), expect: true},
		{name: "uint16", value: uint16(1), expect: true},
		{name: "uint32", value: uint32(1), expect: true},
		{name: "uint64", value: uint64(1), expect: true},
		{name: "float32", value: float32(1.5), expect: true},
		{name: "float64", value: float64(1.5), expect: true},
		{name: "string", value: "1", expect: false},
		{name: "bool", value: true, expect: false},
		{name: "nil", value: nil, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isJSONNumberType(c.value)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestFirstManifest(t *testing.T) {
	items := []model.ExtensionRelease{
		{ManifestJSON: datatypes.JSON([]byte(`{"version":"1.0.0"}`))},
		{ManifestJSON: datatypes.JSON([]byte(`{"version":"2.0.0"}`))},
	}
	got := firstManifest(items)
	if got != `{"version":"1.0.0"}` {
		t.Fatalf("expected first manifest, got: %s", got)
	}
	emptyGot := firstManifest([]model.ExtensionRelease{})
	if emptyGot != "{}" {
		t.Fatalf("expected empty object for empty slice, got: %s", emptyGot)
	}
}

func TestCatalogListQuery(t *testing.T) {
	req := ExtensionCatalogListRequest{
		Keyword:  "test",
		Kind:     "analytics",
		Status:   "active",
		Page:     2,
		PageSize: 50,
	}
	q := catalogListQuery(req)
	if q.Keyword != "test" {
		t.Fatalf("unexpected keyword: %s", q.Keyword)
	}
	if q.Kind != "analytics" {
		t.Fatalf("unexpected kind: %s", q.Kind)
	}
	if q.Status != "active" {
		t.Fatalf("unexpected status: %s", q.Status)
	}
	if q.Limit != 50 {
		t.Fatalf("unexpected limit: %d", q.Limit)
	}
	if q.Offset != 50 {
		t.Fatalf("unexpected offset: %d", q.Offset)
	}
	defaultReq := ExtensionCatalogListRequest{}
	defaultQ := catalogListQuery(defaultReq)
	if defaultQ.Limit != 20 {
		t.Fatalf("unexpected default limit: %d", defaultQ.Limit)
	}
	if defaultQ.Offset != 0 {
		t.Fatalf("unexpected default offset: %d", defaultQ.Offset)
	}
	maxReq := ExtensionCatalogListRequest{PageSize: 200}
	maxQ := catalogListQuery(maxReq)
	if maxQ.Limit != 100 {
		t.Fatalf("unexpected max limit: %d", maxQ.Limit)
	}
}

func TestInstallationListQuery(t *testing.T) {
	req := ExtensionInstallationListRequest{
		ExtensionID: "ext1",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "agent1",
		Status:      "enabled",
		Page:        3,
		PageSize:    25,
	}
	q := installationListQuery(req)
	if q.ExtensionID != "ext1" {
		t.Fatalf("unexpected extension_id: %s", q.ExtensionID)
	}
	if q.Limit != 25 {
		t.Fatalf("unexpected limit: %d", q.Limit)
	}
	if q.Offset != 50 {
		t.Fatalf("unexpected offset: %d", q.Offset)
	}
	defaultReq := ExtensionInstallationListRequest{}
	defaultQ := installationListQuery(defaultReq)
	if defaultQ.Limit != 20 {
		t.Fatalf("unexpected default limit: %d", defaultQ.Limit)
	}
	if defaultQ.Offset != 0 {
		t.Fatalf("unexpected default offset: %d", defaultQ.Offset)
	}
}

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
	item := toInstallationItem(modelItem)
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

func TestPtrInstallationItem(t *testing.T) {
	modelItem := model.ExtensionInstallation{
		Model:         gorm.Model{ID: 456},
		ExtensionID:   "test.ext",
		ReleaseVersion: "2.0.0",
	}
	ptr := ptrInstallationItem(modelItem)
	if ptr == nil {
		t.Fatalf("expected non-nil pointer")
	}
	if ptr.ID != 456 {
		t.Fatalf("unexpected id: %d", ptr.ID)
	}
}

func TestToEventItems(t *testing.T) {
	modelEvents := []model.ExtensionEvent{
		{
			Model:       gorm.Model{ID: 1},
			EventType:   "install.started",
			Level:       "info",
			Message:     "Starting installation",
			PayloadJSON: datatypes.JSON([]byte(`{"step":"download"}`)),
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
			BindingType:  "capability",
			BindingKey:   "analytics.query",
			TargetRef:    "agent1",
			Status:       "active",
			LastError:    "",
		},
		{
			BindingType:  "function",
			BindingKey:   "external.onepanel.upgrade",
			TargetRef:    "agent2",
			Status:       "error",
			LastError:    "connection failed",
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

func TestParseSemVersion(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantValid  bool
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantParts  int
	}{
		{name: "full_version", input: "1.2.3", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "two_parts", input: "1.2", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 0, wantParts: 2},
		{name: "one_part", input: "1", wantValid: true, wantMajor: 1, wantMinor: 0, wantPatch: 0, wantParts: 1},
		{name: "with_v_prefix", input: "v1.2.3", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "with_build_meta", input: "1.2.3+build123", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "with_pre_release", input: "1.2.3-alpha", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "empty_string", input: "", wantValid: false},
		{name: "too_many_parts", input: "1.2.3.4", wantValid: false},
		{name: "no_parts", input: "...", wantValid: false},
		{name: "whitespace_only", input: "   ", wantValid: false},
		{name: "non_numeric", input: "a.b.c", wantValid: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseSemVersion(c.input)
			if ok != c.wantValid {
				t.Fatalf("validity mismatch, got=%v want=%v", ok, c.wantValid)
			}
			if ok {
				if got.major != c.wantMajor {
					t.Fatalf("major mismatch, got=%d want=%d", got.major, c.wantMajor)
				}
				if got.minor != c.wantMinor {
					t.Fatalf("minor mismatch, got=%d want=%d", got.minor, c.wantMinor)
				}
				if got.patch != c.wantPatch {
					t.Fatalf("patch mismatch, got=%d want=%d", got.patch, c.wantPatch)
				}
				if got.parts != c.wantParts {
					t.Fatalf("parts mismatch, got=%d want=%d", got.parts, c.wantParts)
				}
			}
		})
	}
}

func TestCompareSemVersion(t *testing.T) {
	cases := []struct {
		name     string
		a        semVersion
		b        semVersion
		expected int
	}{
		{name: "equal", a: semVersion{major: 1, minor: 2, patch: 3}, b: semVersion{major: 1, minor: 2, patch: 3}, expected: 0},
		{name: "major_greater", a: semVersion{major: 2, minor: 0, patch: 0}, b: semVersion{major: 1, minor: 9, patch: 9}, expected: 1},
		{name: "major_lesser", a: semVersion{major: 1, minor: 9, patch: 9}, b: semVersion{major: 2, minor: 0, patch: 0}, expected: -1},
		{name: "minor_greater", a: semVersion{major: 1, minor: 3, patch: 0}, b: semVersion{major: 1, minor: 2, patch: 9}, expected: 1},
		{name: "minor_lesser", a: semVersion{major: 1, minor: 2, patch: 9}, b: semVersion{major: 1, minor: 3, patch: 0}, expected: -1},
		{name: "patch_greater", a: semVersion{major: 1, minor: 2, patch: 4}, b: semVersion{major: 1, minor: 2, patch: 3}, expected: 1},
		{name: "patch_lesser", a: semVersion{major: 1, minor: 2, patch: 3}, b: semVersion{major: 1, minor: 2, patch: 4}, expected: -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compareSemVersion(c.a, c.b)
			if got != c.expected {
				t.Fatalf("unexpected result, got=%d expected=%d", got, c.expected)
			}
		})
	}
}

func TestMatchSingleClause(t *testing.T) {
	cases := []struct {
		name    string
		current string
		clause  string
		expect  bool
	}{
		{name: "exact_match", current: "1.2.3", clause: "1.2.3", expect: true},
		{name: "exact_no_match", current: "1.2.4", clause: "1.2.3", expect: false},
		{name: "greater_than_true", current: "1.3.0", clause: ">1.2.3", expect: true},
		{name: "greater_than_false", current: "1.2.0", clause: ">1.2.3", expect: false},
		{name: "caret_same_major", current: "1.2.0", clause: "^1.0.0", expect: true},
		{name: "caret_next_major", current: "2.0.0", clause: "^1.2.3", expect: false},
		{name: "tilde_two_parts", current: "1.2.9", clause: "~1.2.0", expect: true},
		{name: "tilde_next_minor", current: "1.3.0", clause: "~1.2.0", expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cur, ok := parseSemVersion(c.current)
			if !ok {
				t.Fatalf("invalid current version: %s", c.current)
			}
			got := matchSingleClause(cur, c.clause)
			if got != c.expect {
				t.Fatalf("unexpected result, current=%s clause=%s got=%v expect=%v", c.current, c.clause, got, c.expect)
			}
		})
	}
}

func TestMatchCaretConstraint(t *testing.T) {
	cases := []struct {
		name    string
		current semVersion
		base    semVersion
		expect  bool
	}{
		{name: "within_range", current: semVersion{major: 1, minor: 5, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "at_base", current: semVersion{major: 1, minor: 2, patch: 3}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "next_major", current: semVersion{major: 2, minor: 0, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
		{name: "below_base", current: semVersion{major: 1, minor: 1, patch: 9}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchCaretConstraint(c.current, c.base)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestMatchTildeConstraint(t *testing.T) {
	cases := []struct {
		name    string
		current semVersion
		base    semVersion
		expect  bool
	}{
		{name: "within_patch_range", current: semVersion{major: 1, minor: 2, patch: 9}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "at_base", current: semVersion{major: 1, minor: 2, patch: 3}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "next_minor", current: semVersion{major: 1, minor: 3, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "below_base", current: semVersion{major: 1, minor: 2, patch: 2}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchTildeConstraint(c.current, c.base)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestNormalizeExtensionID(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"official.analytics", "official.analytics"},
		{"Official.Analytics", "official.analytics"},
		{"  official.analytics  ", "official.analytics"},
		{"OFFICIAL.ANALYTICS", "official.analytics"},
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := normalizeExtensionID(c.input)
			if got != c.expected {
				t.Fatalf("unexpected result, input=%s got=%s expected=%s", c.input, got, c.expected)
			}
		})
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
	disabled := model.ExtensionInstallation{Status: "disabled", DesiredState: "disabled"}
	if !isActiveInstallation(disabled) {
		t.Fatalf("expected disabled installation to be active")
	}
}

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

func TestValidateConfigField(t *testing.T) {
	// Test enum validation
	schema := map[string]any{"enum": []any{"a", "b", "c"}}
	err := validateConfigField("test", "a", schema)
	if err != nil {
		t.Fatalf("unexpected error for valid enum: %v", err)
	}

	err = validateConfigField("test", "d", schema)
	if err == nil {
		t.Fatalf("expected enum validation error")
	}

	// Test type validation
	stringSchema := map[string]any{"type": "string"}
	err = validateConfigField("test", "hello", stringSchema)
	if err != nil {
		t.Fatalf("unexpected error for string type: %v", err)
	}

	err = validateConfigField("test", 123, stringSchema)
	if err == nil {
		t.Fatalf("expected type error for non-string")
	}

	// Test number type
	numberSchema := map[string]any{"type": "number"}
	err = validateConfigField("test", float64(1.5), numberSchema)
	if err != nil {
		t.Fatalf("unexpected error for number type: %v", err)
	}

	// Test integer type
	intSchema := map[string]any{"type": "integer"}
	err = validateConfigField("test", 123, intSchema)
	if err != nil {
		t.Fatalf("unexpected error for integer type: %v", err)
	}

	err = validateConfigField("test", float64(1.5), intSchema)
	if err == nil {
		t.Fatalf("expected integer type error for float")
	}
}

func TestParseStringSliceAny(t *testing.T) {
	cases := []struct {
		name  string
		input []any
		expect []string
	}{
		{name: "strings", input: []any{"a", "b", "c"}, expect: []string{"a", "b", "c"}},
		{name: "mixed", input: []any{"a", 123, "c"}, expect: []string{"a", "123", "c"}},
		{name: "empty", input: []any{}, expect: []string{}},
		{name: "nil", input: nil, expect: []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseStringSliceAny(c.input)
			if len(got) != len(c.expect) {
				t.Fatalf("length mismatch, got=%d want=%d", len(got), len(c.expect))
			}
			for i := range got {
				if got[i] != c.expect[i] {
					t.Fatalf("value mismatch at index %d, got=%s want=%s", i, got[i], c.expect[i])
				}
			}
		})
	}
}

func TestParseStringMapAny(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]any
		expect map[string]string
	}{
		{name: "strings", input: map[string]any{"a": "1", "b": "2"}, expect: map[string]string{"a": "1", "b": "2"}},
		{name: "mixed", input: map[string]any{"a": 123, "b": true}, expect: map[string]string{"a": "123", "b": "true"}},
		{name: "empty", input: map[string]any{}, expect: map[string]string{}},
		{name: "nil", input: nil, expect: map[string]string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseStringMapAny(c.input)
			if len(got) != len(c.expect) {
				t.Fatalf("length mismatch, got=%d want=%d", len(got), len(c.expect))
			}
			for k, v := range c.expect {
				if got[k] != v {
					t.Fatalf("value mismatch for key %s, got=%s want=%s", k, got[k], v)
				}
			}
		})
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

func TestService_FindActiveInstallationByExtension(t *testing.T) {
	t.Skip("Requires full service context setup")
}

func TestService_ActiveInstalledExtensionSet(t *testing.T) {
	t.Skip("Requires full service context setup")
}

func TestService_ResolveInstallationID(t *testing.T) {
	t.Skip("Requires full service context setup")
}

func TestExtractCapabilityDetailsFromBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "capability",
			BindingKey:  "notifications.management",
			SpecJSON:    datatypes.JSON([]byte(`{"operations":["get","update"],"permissions":{"get":"notifications.read","update":"notifications.operate"},"config_keys":["enabled","channels"]}`)),
		},
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			SpecJSON:    datatypes.JSON([]byte(`{"provider":"onepanel","operations":["list_apps","install_app"]}`)),
		},
		{
			BindingType: "function",
			BindingKey:  "external.onepanel.upgrade_app",
		},
	}
	caps, details := extractCapabilityDetailsFromBindings(bindings)
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
	var notify *ExtensionCapabilityDetail
	var onepanel *ExtensionCapabilityDetail
	for i := range details {
		if details[i].Capability == "notifications.management" {
			notify = &details[i]
		}
		if details[i].Capability == "external.onepanel" {
			onepanel = &details[i]
		}
	}
	if notify == nil {
		t.Fatalf("expected notifications.management detail, got: %+v", details)
	}
	if notify.Permissions["update"] != "notifications.operate" {
		t.Fatalf("expected parsed permissions in capability detail, got: %+v", notify.Permissions)
	}
	if len(notify.ConfigKeys) != 2 {
		t.Fatalf("expected parsed config_keys in capability detail, got: %+v", notify.ConfigKeys)
	}
	if onepanel == nil {
		t.Fatalf("expected external.onepanel detail, got: %+v", details)
	}
	if len(onepanel.Operations) != 3 {
		t.Fatalf("expected merged operations [list_apps install_app upgrade_app], got: %+v", onepanel.Operations)
	}
}

func TestExtractPageDetailsFromBindings(t *testing.T) {
	pages := extractPageDetailsFromBindings([]model.ExtensionRuntimeBinding{
		{
			BindingType: "page",
			BindingKey:  "analytics.realtime",
			SpecJSON:    datatypes.JSON([]byte(`{"title":"Realtime","route":"/analytics/realtime","order":20}`)),
		},
		{
			BindingType: "page",
			BindingKey:  "analytics.overview",
			SpecJSON:    datatypes.JSON([]byte(`{"title":"Overview","route":"/analytics/overview","order":10,"required_permission":"analytics.read"}`)),
		},
		{
			BindingType: "navigation",
			BindingKey:  "analytics.behavior",
			SpecJSON:    datatypes.JSON([]byte(`{"title":"Behavior","route":"/analytics/behavior","order":30}`)),
		},
	})
	if len(pages) != 3 {
		t.Fatalf("expected 3 page items, got %d", len(pages))
	}
	var overview *ExtensionPageItem
	for i := range pages {
		if pages[i].Key == "analytics.overview" {
			overview = &pages[i]
			break
		}
	}
	if overview == nil || overview.RequiredPermission != "analytics.read" {
		t.Fatalf("expected required_permission=analytics.read, got: %+v", overview)
	}
}

func TestNewHandler(t *testing.T) {
	svc := &Service{}
	h := NewHandler(svc)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.service != svc {
		t.Fatal("expected service to be set")
	}
}

func TestHandler_Operator(t *testing.T) {
	svc := &Service{}
	h := NewHandler(svc)

	// Mock gin context
	c := &gin.Context{}
	c.Set("username", "testuser")
	op := h.operator(c)
	if op != "testuser" {
		t.Fatalf("unexpected operator: %s", op)
	}

	c.Set("username", "")
	op = h.operator(c)
	if op != "system" {
		t.Fatalf("unexpected operator for empty username: %s", op)
	}

	c2 := &gin.Context{}
	op = h.operator(c2)
	if op != "system" {
		t.Fatalf("unexpected operator for no username: %s", op)
	}
}

func TestParseUintParam(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		expectID  uint
		expectErr bool
	}{
		{name: "valid", value: "123", expectID: 123, expectErr: false},
		{name: "zero", value: "0", expectID: 0, expectErr: false},
		{name: "large", value: "4294967295", expectID: 4294967295, expectErr: false},
		{name: "invalid", value: "abc", expectErr: true},
		{name: "negative", value: "-1", expectErr: true},
		{name: "float", value: "1.5", expectErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := &gin.Context{}
			ctx.Params = gin.Params{gin.Param{Key: "id", Value: c.value}}
			id, err := parseUintParam(ctx, "id")
			if c.expectErr {
				if err == nil {
					t.Fatalf("expected error for value: %s", c.value)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if id != c.expectID {
					t.Fatalf("unexpected id: got=%d expect=%d", id, c.expectID)
				}
			}
		})
	}
}

func TestValidateConfigAgainstSchemaNilConfig(t *testing.T) {
	schema := map[string]any{
		"required": []any{"field1"},
		"properties": map[string]any{
			"field1": map[string]any{"type": "string"},
		},
	}
	// nil config should be treated as empty map
	err := validateConfigAgainstSchema(nil, schema)
	if err == nil {
		t.Fatalf("expected error for missing required field")
	}
}

func TestValidateConfigFieldEmptySchema(t *testing.T) {
	// Empty schema should not error
	err := validateConfigField("test", "anyvalue", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error for empty schema: %v", err)
	}
}

func TestValidateConfigFieldNoType(t *testing.T) {
	schema := map[string]any{} // no type specified
	err := validateConfigField("test", "anyvalue", schema)
	if err != nil {
		t.Fatalf("unexpected error for schema without type: %v", err)
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

func TestFirstManifestEmptyReleases(t *testing.T) {
	got := firstManifest([]model.ExtensionRelease{})
	if got != "{}" {
		t.Fatalf("expected empty object for empty releases, got: %s", got)
	}
}

func TestValidateConfigFieldBoolean(t *testing.T) {
	schema := map[string]any{"type": "boolean"}
	err := validateConfigField("test", true, schema)
	if err != nil {
		t.Fatalf("unexpected error for boolean: %v", err)
	}

	err = validateConfigField("test", "true", schema)
	if err == nil {
		t.Fatalf("expected error for string as boolean")
	}

	err = validateConfigField("test", 1, schema)
	if err == nil {
		t.Fatalf("expected error for int as boolean")
	}
}

func TestValidateConfigFieldObject(t *testing.T) {
	schema := map[string]any{"type": "object"}
	valid := map[string]any{"key": "value"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for object: %v", err)
	}

	err = validateConfigField("test", "not-an-object", schema)
	if err == nil {
		t.Fatalf("expected error for string as object")
	}
}

func TestValidateConfigFieldArray(t *testing.T) {
	schema := map[string]any{"type": "array"}
	valid := []any{"item1", "item2"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for array: %v", err)
	}

	err = validateConfigField("test", "not-an-array", schema)
	if err == nil {
		t.Fatalf("expected error for string as array")
	}
}

func TestValidateConfigFieldEnum(t *testing.T) {
	schema := map[string]any{"enum": []any{"option1", "option2"}}
	err := validateConfigField("test", "option1", schema)
	if err != nil {
		t.Fatalf("unexpected error for valid enum value: %v", err)
	}

	err = validateConfigField("test", "option3", schema)
	if err == nil {
		t.Fatalf("expected error for invalid enum value")
	}

	// Empty enum should not error
	emptySchema := map[string]any{"enum": []any{}}
	err = validateConfigField("test", "any", emptySchema)
	if err != nil {
		t.Fatalf("unexpected error for empty enum: %v", err)
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

func TestNewService(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	s := NewService(svcCtx)
	if s == nil {
		t.Fatal("expected non-nil service")
	}
	if s.svcCtx != svcCtx {
		t.Fatal("expected svcCtx to be set")
	}
}
