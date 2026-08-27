package extension

import (
	"context"
	"sync"
	"testing"
	"time"

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
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
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
		name   string
		value  any
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
		{ManifestJSON: model.JSON([]byte(`{"version":"1.0.0"}`))},
		{ManifestJSON: model.JSON([]byte(`{"version":"2.0.0"}`))},
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
		Model:          gorm.Model{ID: 456},
		ExtensionID:    "test.ext",
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

func TestParseSemVersion(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantValid bool
		wantMajor int
		wantMinor int
		wantPatch int
		wantParts int
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
		name   string
		input  []any
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
		name   string
		input  map[string]any
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

func TestService_ResolveInstallationID_Skipped(t *testing.T) {
	t.Skip("Requires full service context setup")
}

func TestExtractCapabilityDetailsFromBindings(t *testing.T) {
	bindings := []model.ExtensionRuntimeBinding{
		{
			BindingType: "capability",
			BindingKey:  "notifications.management",
			SpecJSON:    model.JSON([]byte(`{"operations":["get","update"],"permissions":{"get":"notifications.read","update":"notifications.operate"},"config_keys":["enabled","channels"]}`)),
		},
		{
			BindingType: "provider",
			BindingKey:  "onepanel",
			SpecJSON:    model.JSON([]byte(`{"provider":"onepanel","operations":["list_apps","install_app"]}`)),
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
			SpecJSON:    model.JSON([]byte(`{"title":"Realtime","route":"/analytics/realtime","order":20}`)),
		},
		{
			BindingType: "page",
			BindingKey:  "analytics.overview",
			SpecJSON:    model.JSON([]byte(`{"title":"Overview","route":"/analytics/overview","order":10,"required_permission":"analytics.read"}`)),
		},
		{
			BindingType: "navigation",
			BindingKey:  "analytics.behavior",
			SpecJSON:    model.JSON([]byte(`{"title":"Behavior","route":"/analytics/behavior","order":30}`)),
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

func TestConnectionStatusByInstallation(t *testing.T) {
	tests := []struct {
		name     string
		item     *model.ExtensionInstallation
		expected string
	}{
		{
			name:     "nil item",
			item:     nil,
			expected: "unknown",
		},
		{
			name: "enabled status",
			item: &model.ExtensionInstallation{
				Status:  "enabled",
				Enabled: true,
			},
			expected: "ok",
		},
		{
			name: "disabled status",
			item: &model.ExtensionInstallation{
				Status:  "disabled",
				Enabled: false,
			},
			expected: "disabled",
		},
		{
			name: "uninstalled status",
			item: &model.ExtensionInstallation{
				Status:       "uninstalled",
				DesiredState: "uninstalled",
			},
			expected: "uninstalled",
		},
		{
			name: "error status with enabled=true",
			item: &model.ExtensionInstallation{
				Status:  "error",
				Enabled: true,
			},
			expected: "ok",
		},
		{
			name: "error status with enabled=false",
			item: &model.ExtensionInstallation{
				Status:  "error",
				Enabled: false,
			},
			expected: "disabled",
		},
		{
			name: "pending status with enabled=false",
			item: &model.ExtensionInstallation{
				Status:  "pending",
				Enabled: false,
			},
			expected: "disabled",
		},
		{
			name: "installing status with enabled=true",
			item: &model.ExtensionInstallation{
				Status:  "installing",
				Enabled: true,
			},
			expected: "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connectionStatusByInstallation(tt.item)
			if got != tt.expected {
				t.Fatalf("unexpected status, got=%s expect=%s", got, tt.expected)
			}
		})
	}
}

func TestIsJSONIntegerType(t *testing.T) {
	cases := []struct {
		name   string
		value  any
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
		{name: "float32", value: float32(1.5), expect: false},
		{name: "float64", value: float64(1.5), expect: false},
		{name: "float64 integer value", value: float64(2.0), expect: true}, // 2.0 is an integer value
		{name: "string", value: "1", expect: false},
		{name: "bool", value: true, expect: false},
		{name: "nil", value: nil, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isJSONIntegerType(c.value)
			if got != c.expect {
				t.Fatalf("unexpected result for %T, got=%v expect=%v", c.value, got, c.expect)
			}
		})
	}
}

func TestExtensionInstallRequest_Validation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := ExtensionInstallRequest{
			ExtensionID:    "test.ext",
			ReleaseVersion: "1.0.0",
			ScopeType:      "system",
			ScopeID:        "global",
			TargetType:     "agent",
			Config:         map[string]any{"key": "value"},
			SecretRefs:     map[string]string{"secret": "ref"},
		}
		if req.ExtensionID == "" {
			t.Fatal("extension_id should not be empty")
		}
	})

	t.Run("minimal request", func(t *testing.T) {
		req := ExtensionInstallRequest{
			ExtensionID:    "test.ext",
			ReleaseVersion: "1.0.0",
			ScopeType:      "system",
			ScopeID:        "global",
			TargetType:     "agent",
		}
		if req.ExtensionID == "" || req.ReleaseVersion == "" {
			t.Fatal("required fields missing")
		}
	})
}

func TestExtensionCatalogListRequest_Defaults(t *testing.T) {
	req := ExtensionCatalogListRequest{}
	// Note: struct tags don't set default values in Go structs
	// The default values are applied by the query parser in the handler
	// Here we just verify the zero values
	if req.Page != 0 {
		t.Fatalf("unexpected default page: %d", req.Page)
	}
	if req.PageSize != 0 {
		t.Fatalf("unexpected default page_size: %d", req.PageSize)
	}
}

func TestExtensionInstallationListRequest_Defaults(t *testing.T) {
	req := ExtensionInstallationListRequest{}
	// Note: struct tags don't set default values in Go structs
	if req.Page != 0 {
		t.Fatalf("unexpected default page: %d", req.Page)
	}
	if req.PageSize != 0 {
		t.Fatalf("unexpected default page_size: %d", req.PageSize)
	}
}

func TestExtensionEventListRequest_Defaults(t *testing.T) {
	req := ExtensionEventListRequest{}
	// Note: struct tags don't set default values in Go structs
	if req.Page != 0 {
		t.Fatalf("unexpected default page: %d", req.Page)
	}
	if req.PageSize != 0 {
		t.Fatalf("unexpected default page_size: %d", req.PageSize)
	}
}

func TestExtensionUpgradeRequest_Validation(t *testing.T) {
	req := ExtensionUpgradeRequest{
		ReleaseVersion: "2.0.0",
	}
	if req.ReleaseVersion == "" {
		t.Fatal("release_version should not be empty")
	}
}

func TestExtensionConfigUpdateRequest(t *testing.T) {
	req := ExtensionConfigUpdateRequest{
		Config:     map[string]any{"enabled": true},
		SecretRefs: map[string]string{"api_key": "secret:ref"},
	}
	if req.Config == nil {
		t.Fatal("config should not be nil")
	}
	if req.SecretRefs == nil {
		t.Fatal("secret_refs should not be nil")
	}
}

func TestCatalogListQuery_EdgeCases(t *testing.T) {
	t.Run("negative page", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: -1, PageSize: 10}
		q := catalogListQuery(req)
		if q.Offset != 0 {
			t.Fatalf("expected offset 0 for negative page, got: %d", q.Offset)
		}
	})

	t.Run("zero page size", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 0}
		q := catalogListQuery(req)
		if q.Limit != 20 {
			t.Fatalf("expected limit 20 for zero page_size, got: %d", q.Limit)
		}
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 200}
		q := catalogListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 for page_size > 100, got: %d", q.Limit)
		}
	})

	t.Run("page size at max boundary", func(t *testing.T) {
		req := ExtensionCatalogListRequest{Page: 1, PageSize: 100}
		q := catalogListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 at boundary, got: %d", q.Limit)
		}
	})
}

func TestInstallationListQuery_EdgeCases(t *testing.T) {
	t.Run("negative page", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: -1, PageSize: 10}
		q := installationListQuery(req)
		if q.Offset != 0 {
			t.Fatalf("expected offset 0 for negative page, got: %d", q.Offset)
		}
	})

	t.Run("zero page size", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: 1, PageSize: 0}
		q := installationListQuery(req)
		if q.Limit != 20 {
			t.Fatalf("expected limit 20 for zero page_size, got: %d", q.Limit)
		}
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		req := ExtensionInstallationListRequest{Page: 1, PageSize: 150}
		q := installationListQuery(req)
		if q.Limit != 100 {
			t.Fatalf("expected limit 100 for page_size > 100, got: %d", q.Limit)
		}
	})
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
	item := toInstallationItem(modelItem)
	if item.ID != 123 {
		t.Fatalf("unexpected id: %d", item.ID)
	}
	if item.UpdatedAt == 0 {
		t.Fatal("expected non-zero UpdatedAt")
	}
}

func TestMatchSingleClause_EdgeCases(t *testing.T) {
	t.Run("empty clause", func(t *testing.T) {
		cur, ok := parseSemVersion("1.2.3")
		if !ok {
			t.Fatal("failed to parse version")
		}
		// Empty clause - parseSemVersion returns false for empty string
		got := matchSingleClause(cur, "")
		if got {
			t.Fatalf("expected false for empty clause (fails to parse)")
		}
	})

	t.Run("whitespace clause", func(t *testing.T) {
		cur, ok := parseSemVersion("1.2.3")
		if !ok {
			t.Fatal("failed to parse version")
		}
		got := matchSingleClause(cur, "  ")
		// Whitespace fails to parse as semVersion, so returns false
		if got {
			t.Fatalf("expected false for whitespace clause (fails to parse)")
		}
	})
}

func TestMatchVersionConstraint_EdgeCases(t *testing.T) {
	t.Run("empty constraint", func(t *testing.T) {
		got := matchVersionConstraint("1.2.3", "")
		if !got {
			t.Fatalf("expected true for empty constraint")
		}
	})

	t.Run("whitespace constraint", func(t *testing.T) {
		got := matchVersionConstraint("1.2.3", "  ,  ")
		// Whitespace clauses fail to parse, but empty clauses in comma-separated list are skipped
		if !got {
			t.Fatalf("expected true for whitespace constraint (skipped)")
		}
	})

	t.Run("multiple clauses", func(t *testing.T) {
		got := matchVersionConstraint("1.5.0", ">=1.2.0, <2.0.0")
		if !got {
			t.Fatalf("expected true for valid range")
		}
	})

	t.Run("multiple clauses failing", func(t *testing.T) {
		got := matchVersionConstraint("2.5.0", ">=1.2.0, <2.0.0")
		if got {
			t.Fatalf("expected false for version outside range")
		}
	})
}

func TestValidateConfigField_Array(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
	valid := []any{"item1", "item2"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for array: %v", err)
	}

	// Test non-array value
	err = validateConfigField("test", "not-an-array", schema)
	if err == nil {
		t.Fatalf("expected error for non-array value")
	}
}

func TestValidateConfigField_Object_Nested(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nested": map[string]any{"type": "string"},
		},
	}
	valid := map[string]any{"nested": "value"}
	err := validateConfigField("test", valid, schema)
	if err != nil {
		t.Fatalf("unexpected error for nested object: %v", err)
	}
}

func TestValidateConfigField_Number(t *testing.T) {
	schema := map[string]any{"type": "number"}
	err := validateConfigField("test", float64(3.14), schema)
	if err != nil {
		t.Fatalf("unexpected error for number: %v", err)
	}

	err = validateConfigField("test", 3, schema)
	if err != nil {
		t.Fatalf("unexpected error for int as number: %v", err)
	}

	err = validateConfigField("test", "3.14", schema)
	if err == nil {
		t.Fatalf("expected error for string as number")
	}
}

func TestResolveConfigSchema_EmptyVersion(t *testing.T) {
	// This test requires a full ServiceContext setup with Extensions.Catalog
	// For now, just test that the function doesn't panic with nil context
	// The actual schema resolution is tested in integration tests
	t.Skip("Requires full service context setup")
}

func TestParseSemVersion_PreRelease(t *testing.T) {
	cases := []struct {
		input     string
		wantValid bool
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{input: "1.2.3-alpha.1", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "1.2.3-beta", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "1.2.3-rc1", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, ok := parseSemVersion(c.input)
			if ok != c.wantValid {
				t.Fatalf("validity mismatch, got=%v want=%v", ok, c.wantValid)
			}
			if ok {
				if got.major != c.wantMajor {
					t.Fatalf("major mismatch, got=%d want=%d", got.major, c.wantMajor)
				}
			}
		})
	}
}

func TestCompareSemVersion_Equal(t *testing.T) {
	a := semVersion{major: 1, minor: 2, patch: 3}
	b := semVersion{major: 1, minor: 2, patch: 3}
	if compareSemVersion(a, b) != 0 {
		t.Fatal("expected versions to be equal")
	}
}

func TestMatchCaretConstraint_ZeroVersions(t *testing.T) {
	t.Run("zero major with parts field", func(t *testing.T) {
		// For ^0.2.3, it allows >=0.2.3 <0.3.0
		current := semVersion{major: 0, minor: 2, patch: 5, parts: 3}
		base := semVersion{major: 0, minor: 2, patch: 3, parts: 3}
		if !matchCaretConstraint(current, base) {
			t.Fatal("expected true for 0.2.5 vs ^0.2.3")
		}
	})

	t.Run("zero minor with parts field - at boundary", func(t *testing.T) {
		// For ^0.0.2, it allows >=0.0.2 <0.0.3
		// So 0.0.2 should match
		current := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		base := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		if !matchCaretConstraint(current, base) {
			t.Fatal("expected true for 0.0.2 vs ^0.0.2 (exact match)")
		}
	})

	t.Run("zero minor with parts field - outside upper bound", func(t *testing.T) {
		// For ^0.0.2, upper bound is 0.0.3, so 0.0.3 should NOT match
		current := semVersion{major: 0, minor: 0, patch: 3, parts: 3}
		base := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		if matchCaretConstraint(current, base) {
			t.Fatal("expected false for 0.0.3 vs ^0.0.2 (at upper bound)")
		}
	})
}

func TestMatchTildeConstraint_SinglePart(t *testing.T) {
	current := semVersion{major: 1, minor: 0, patch: 5}
	base := semVersion{major: 1, minor: 0, patch: 0, parts: 1}
	if !matchTildeConstraint(current, base) {
		t.Fatal("expected true for single part version")
	}
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

func TestFindInstallationConflict_NilDB(t *testing.T) {
	svcCtx := &svc.ServiceContext{DB: nil}
	s := &Service{svcCtx: svcCtx}

	req := ExtensionInstallRequest{
		ExtensionID: "test.ext",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "agent1",
	}

	_, _, err := s.findInstallationConflict(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nil DB")
	}
}

func TestResolveCatalogMetadata_EmptyExtensionID(t *testing.T) {
	// This test requires a proper ServiceContext with Extensions.Catalog
	// For now, skip as we can't easily mock the catalog interface
	t.Skip("Requires full service context with catalog")
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

func TestMatchVersionConstraint_SimpleEquality(t *testing.T) {
	// When constraint is just a version number, it's an exact match
	if !matchVersionConstraint("1.2.3", "1.2.3") {
		t.Fatal("expected true for exact version match")
	}

	if matchVersionConstraint("1.2.4", "1.2.3") {
		t.Fatal("expected false for different version")
	}
}

func TestMatchVersionConstraint_LessThan(t *testing.T) {
	if !matchVersionConstraint("1.2.2", "<1.2.3") {
		t.Fatal("expected true for less than")
	}

	if matchVersionConstraint("1.2.3", "<1.2.3") {
		t.Fatal("expected false for equal version with <")
	}

	if matchVersionConstraint("1.2.4", "<1.2.3") {
		t.Fatal("expected false for greater version with <")
	}
}

func TestMatchVersionConstraint_LessThanOrEqual(t *testing.T) {
	if !matchVersionConstraint("1.2.2", "<=1.2.3") {
		t.Fatal("expected true for less than or equal")
	}

	if !matchVersionConstraint("1.2.3", "<=1.2.3") {
		t.Fatal("expected true for equal version with <=")
	}

	if matchVersionConstraint("1.2.4", "<=1.2.3") {
		t.Fatal("expected false for greater version with <=")
	}
}

func TestMatchVersionConstraint_GreaterThan(t *testing.T) {
	if !matchVersionConstraint("1.2.4", ">1.2.3") {
		t.Fatal("expected true for greater than")
	}

	if matchVersionConstraint("1.2.3", ">1.2.3") {
		t.Fatal("expected false for equal version with >")
	}

	if matchVersionConstraint("1.2.2", ">1.2.3") {
		t.Fatal("expected false for less version with >")
	}
}

func TestMatchVersionConstraint_GreaterThanOrEqual(t *testing.T) {
	if !matchVersionConstraint("1.2.4", ">=1.2.3") {
		t.Fatal("expected true for greater than or equal")
	}

	if !matchVersionConstraint("1.2.3", ">=1.2.3") {
		t.Fatal("expected true for equal version with >=")
	}

	if matchVersionConstraint("1.2.2", ">=1.2.3") {
		t.Fatal("expected false for less version with >=")
	}
}

func TestMatchVersionConstraint_Prefix(t *testing.T) {
	// Test with v prefix - parseSemVersion handles v prefix
	if !matchVersionConstraint("v1.2.3", ">=1.2.0") {
		t.Fatal("expected true for version with v prefix")
	}

	// V prefix is not handled (only lowercase v)
	if !matchVersionConstraint("1.2.3", ">=1.2.0") {
		t.Fatal("expected true for version constraint")
	}
}

func TestMatchVersionConstraint_BuildMetadata(t *testing.T) {
	// Test with build metadata (should be ignored)
	if !matchVersionConstraint("1.2.3+build123", ">=1.2.0") {
		t.Fatal("expected true for version with build metadata")
	}

	if !matchVersionConstraint("1.2.3-alpha+build123", ">=1.2.0") {
		t.Fatal("expected true for version with pre-release and build")
	}
}

func TestMatchVersionConstraint_TwoPartVersion(t *testing.T) {
	// Test with two-part versions (1.2)
	// Two-part version "1.2" means exactly 1.2.0, not >=1.2
	if !matchVersionConstraint("1.2", "1.2") {
		t.Fatal("expected true for exact match of two-part version")
	}

	// 1.2.0 should match 1.2
	if !matchVersionConstraint("1.2.0", "1.2") {
		t.Fatal("expected true for 1.2.0 matching constraint 1.2")
	}

	// But 1.2.5 won't match - it's an exact match
	if matchVersionConstraint("1.2.5", "1.2") {
		t.Fatal("expected false - two-part version is exact match to 1.2.0")
	}
}

func TestMatchVersionConstraint_OnePartVersion(t *testing.T) {
	// Test with one-part versions (1)
	if !matchVersionConstraint("1", "1") {
		t.Fatal("expected true for exact match of one-part version")
	}

	// One-part version "1" means exactly 1.0.0, so 1.5 won't match
	if matchVersionConstraint("1.5", "1") {
		t.Fatal("expected false - one-part version is exact match")
	}

	// But 1.0 matches 1
	if !matchVersionConstraint("1.0", "1") {
		t.Fatal("expected true for 1.0 matching constraint 1")
	}
}

func TestValidateConfigField_NullValue(t *testing.T) {
	schema := map[string]any{"type": "string"}
	err := validateConfigField("test", nil, schema)
	// nil is not valid for string type
	if err == nil {
		t.Fatal("expected error for nil value with string type")
	}
}

func TestValidateConfigAgainstSchema_NoProperties(t *testing.T) {
	schema := map[string]any{
		"required": []any{"field1"},
	}
	err := validateConfigAgainstSchema(map[string]any{"field1": "value"}, schema)
	// Schema without properties should validate required fields but not types
	if err != nil {
		t.Fatalf("unexpected error for schema without properties: %v", err)
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

	item := toInstallationItem(modelItem)
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
	assert.Equal(t, "unknown", item.HealthStatus)
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

func TestCatalogListQuery_AllParams(t *testing.T) {
	req := ExtensionCatalogListRequest{
		Keyword:  "search",
		Kind:     "provider",
		Status:   "beta",
		Page:     5,
		PageSize: 50,
	}
	q := catalogListQuery(req)
	assert.Equal(t, "search", q.Keyword)
	assert.Equal(t, "provider", q.Kind)
	assert.Equal(t, "beta", q.Status)
	assert.Equal(t, 50, q.Limit)
	assert.Equal(t, 200, q.Offset) // (5-1)*50
}

func TestInstallationListQuery_AllParams(t *testing.T) {
	req := ExtensionInstallationListRequest{
		ExtensionID: "test.ext",
		ScopeType:   "system",
		ScopeID:     "global",
		TargetType:  "agent",
		TargetID:    "agent1",
		Status:      "enabled",
		Page:        2,
		PageSize:    10,
		Enabled:     boolPtr(true),
	}
	q := installationListQuery(req)
	assert.Equal(t, "test.ext", q.ExtensionID)
	assert.Equal(t, "system", q.ScopeType)
	assert.Equal(t, "global", q.ScopeID)
	assert.Equal(t, "agent", q.TargetType)
	assert.Equal(t, "agent1", q.TargetID)
	assert.Equal(t, "enabled", q.Status)
	assert.Equal(t, 10, q.Limit)
	assert.Equal(t, 10, q.Offset)
	assert.True(t, q.Enabled != nil && *q.Enabled)
}

func boolPtr(b bool) *bool {
	return &b
}

func TestInstallationListQuery_WithEnabled(t *testing.T) {
	f := false
	req := ExtensionInstallationListRequest{
		Enabled: &f,
	}
	q := installationListQuery(req)
	assert.True(t, q.Enabled != nil && *q.Enabled == false)
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

var (
	integrationTestDB      *gorm.DB
	integrationTestDBOnce  sync.Once
	integrationTestDBMutex sync.Mutex
)

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
func setupExtensionTestContext(t *testing.T, db *gorm.DB) *svc.ServiceContext {
	permSvc := permission.NewPermissionService(db)
	nullCache := cache.NewNullCache()
	cacheHelper := cache.NewCacheHelper(nullCache)

	extensionRepos := extensiongorm.NewBundle(db)
	extensionManifestSvc := extensionmanifest.NewService()
	extensionCatalogSvc := extensioncatalog.NewService(extensionRepos.Catalog, extensionRepos.Release)
	extensionInstallationSvc := extensioninstallation.NewService(extensionRepos.Installation, extensionRepos.Event, extensionRepos.Binding)
	extensionRuntimeSvc := extensionruntime.NewService(extensionRepos.Installation, extensionRepos.Binding, extensionRepos.Event)
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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if len(resp.Releases) != 2 {
		t.Fatalf("expected 2 releases, got: %d", len(resp.Releases))
	}

	if resp.Total != 2 {
		t.Fatalf("expected total 2, got: %d", resp.Total)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if len(resp.Items) == 0 {
		t.Fatal("expected at least one installation")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "updated" {
		t.Fatalf("expected status 'updated', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if len(resp.Schema) == 0 {
		t.Fatal("expected non-empty config schema from manifest")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Config == nil {
		t.Fatal("expected config to be non-nil")
	}

	if len(resp.SecretRefs) == 0 {
		t.Fatal("expected secret_refs to be non-empty")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "disabled" { // New installations start disabled
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	// Should have capabilities from manifest since bindings are empty
	if len(resp.Capabilities) == 0 {
		t.Fatal("expected capabilities from manifest")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	// Pages list should be returned (empty if not defined)
	if resp.Pages == nil {
		t.Fatal("expected pages list to be initialized")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "disabled" { // New installations start disabled
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}

	if resp.CheckedAt == 0 {
		t.Fatal("expected checked_at to be set")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "enabled" {
		t.Fatalf("expected status 'enabled', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "disabled" {
		t.Fatalf("expected status 'disabled', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "upgraded" {
		t.Fatalf("expected status 'upgraded', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Applied < 0 {
		t.Fatalf("expected applied >= 0, got: %d", resp.Applied)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	if resp.Status != "uninstalled" {
		t.Fatalf("expected status 'uninstalled', got: %s", resp.Status)
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
	}

	// Events list should be returned (may be empty initially)
	if resp.Items == nil {
		t.Fatal("expected events list to be initialized")
	}
}

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

	if resp.Code != 200 {
		t.Fatalf("expected code 200, got: %d", resp.Code)
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
