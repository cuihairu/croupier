package extension

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
)

func TestParseStringSliceAny_V8(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect []string
	}{
		{"nil", nil, []string{}},
		{"not a slice", "hello", []string{}},
		{"empty slice", []any{}, []string{}},
		{"strings", []any{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with spaces", []any{"  a  ", " b "}, []string{"a", "b"}},
		{"empty strings filtered", []any{"", " ", "a"}, []string{"a"}},
		{"nil values filtered", []any{nil, "a"}, []string{"a"}},
		{"duplicates removed", []any{"a", "b", "a"}, []string{"a", "b"}},
		{"mixed types", []any{"a", 1, true}, []string{"a", "1", "true"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStringSliceAny(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("got %v, want %v", got, tc.expect)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Fatalf("got[%d]=%q, want %q", i, got[i], tc.expect[i])
				}
			}
		})
	}
}

func TestParseStringMapAny_V8(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect map[string]string
	}{
		{"nil", nil, map[string]string{}},
		{"not a map", "hello", map[string]string{}},
		{"empty map", map[string]any{}, map[string]string{}},
		{"valid map", map[string]any{"a": "1", "b": "2"}, map[string]string{"a": "1", "b": "2"}},
		{"with spaces", map[string]any{" a ": " 1 "}, map[string]string{"a": "1"}},
		{"empty key filtered", map[string]any{"": "1"}, map[string]string{}},
		{"nil value filtered", map[string]any{"a": nil}, map[string]string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStringMapAny(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("got %v, want %v", got, tc.expect)
			}
			for k, v := range tc.expect {
				if got[k] != v {
					t.Fatalf("got[%s]=%q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestFirstManifest_V8_Empty(t *testing.T) {
	got := firstManifest([]model.ExtensionRelease{})
	if got != "{}" {
		t.Fatalf("expected '{}', got %q", got)
	}
}

func TestFirstManifest_V8_WithData(t *testing.T) {
	items := []model.ExtensionRelease{
		{ManifestJSON: []byte(`{"name":"test"}`)},
		{ManifestJSON: []byte(`{"name":"other"}`)},
	}
	got := firstManifest(items)
	if got != `{"name":"test"}` {
		t.Fatalf("expected first manifest, got %q", got)
	}
}

func TestMapString_V8(t *testing.T) {
	tests := []struct {
		name   string
		m      map[string]any
		key    string
		expect string
	}{
		{"nil map", nil, "key", ""},
		{"missing key", map[string]any{"other": "val"}, "key", ""},
		{"nil value", map[string]any{"key": nil}, "key", ""},
		{"valid value", map[string]any{"key": "hello"}, "key", "hello"},
		{"spaces", map[string]any{"key": "  hello  "}, "key", "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapString(tc.m, tc.key)
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestNormalizeExtensionID_V8(t *testing.T) {
	tests := []struct {
		input, expect string
	}{
		{"MyExt", "myext"},
		{"  MyExt  ", "myext"},
		{"", ""},
		{"ABC-123", "abc-123"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
	}
	for _, tc := range tests {
		got := normalizeExtensionID(tc.input)
		if got != tc.expect {
			t.Errorf("normalizeExtensionID(%q) = %q, want %q", tc.input, got, tc.expect)
		}
	}
}

func TestExtractCapabilities_V8(t *testing.T) {
	got := extractCapabilities(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	got = extractCapabilities(map[string]any{"name": "test"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	got = extractCapabilities(map[string]any{"capabilities": "single"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	manifest := map[string]any{
		"capabilities": []any{
			"read", "write",
			map[string]any{"id": "admin", "name": "Admin"},
			"read", "",
		},
	}
	got = extractCapabilities(manifest)
	if len(got) != 3 {
		t.Fatalf("expected 3 capabilities, got %v", got)
	}
}

func TestExtractTags_V8(t *testing.T) {
	got := extractTags(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	got = extractTags(map[string]any{"tags": "single"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	got = extractTags(map[string]any{"tags": []any{"a", "b", "a", ""}})
	if len(got) != 2 {
		t.Fatalf("expected 2 tags, got %v", got)
	}
}

func TestExtractDefaultInstall_V8(t *testing.T) {
	tests := []struct {
		name     string
		manifest map[string]any
		expect   bool
	}{
		{"nil", nil, false},
		{"no key", map[string]any{}, false},
		{"bool true", map[string]any{"default_install": true}, true},
		{"bool false", map[string]any{"default_install": false}, false},
		{"string true", map[string]any{"default_install": "true"}, true},
		{"string yes", map[string]any{"default_install": "yes"}, true},
		{"string 1", map[string]any{"default_install": "1"}, true},
		{"string no", map[string]any{"default_install": "no"}, false},
		{"camelCase", map[string]any{"defaultInstall": true}, true},
		{"float64 non-zero", map[string]any{"default_install": float64(1)}, true},
		{"float64 zero", map[string]any{"default_install": float64(0)}, false},
		{"int non-zero", map[string]any{"default_install": int(1)}, true},
		{"int zero", map[string]any{"default_install": int(0)}, false},
		{"unknown type", map[string]any{"default_install": struct{}{}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDefaultInstall(tc.manifest)
			if got != tc.expect {
				t.Errorf("got %v, want %v", got, tc.expect)
			}
		})
	}
}

func TestConnectionStatusByInstallation_V8(t *testing.T) {
	tests := []struct {
		name   string
		item   *model.ExtensionInstallation
		expect string
	}{
		{"nil", nil, "unknown"},
		{"uninstalled", &model.ExtensionInstallation{Status: "uninstalled"}, "uninstalled"},
		{"desired uninstalled", &model.ExtensionInstallation{DesiredState: "uninstalled"}, "uninstalled"},
		{"enabled", &model.ExtensionInstallation{Status: "active", Enabled: true}, "ok"},
		{"disabled", &model.ExtensionInstallation{Status: "active", Enabled: false}, "disabled"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := connectionStatusByInstallation(tc.item)
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestIsActiveInstallation_V8(t *testing.T) {
	if isActiveInstallation(model.ExtensionInstallation{Status: "uninstalled"}) {
		t.Fatal("expected false for uninstalled status")
	}
	if isActiveInstallation(model.ExtensionInstallation{DesiredState: "uninstalled"}) {
		t.Fatal("expected false for uninstalled desired state")
	}
	if !isActiveInstallation(model.ExtensionInstallation{Status: "active", DesiredState: "running"}) {
		t.Fatal("expected true for active installation")
	}
}

func TestFormatDependentRef_V8(t *testing.T) {
	tests := []struct {
		name   string
		item   model.ExtensionInstallation
		expect string
	}{
		{"normal", model.ExtensionInstallation{ExtensionID: "ext-a", ReleaseVersion: "1.0.0"}, "ext-a@1.0.0"},
		{"empty id", model.ExtensionInstallation{ExtensionID: "", ReleaseVersion: "1.0.0"}, "unknown@1.0.0"},
		{"empty ver", model.ExtensionInstallation{ExtensionID: "ext-a", ReleaseVersion: ""}, "ext-a"},
		{"both empty", model.ExtensionInstallation{ExtensionID: "", ReleaseVersion: ""}, "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatDependentRef(tc.item)
			if got != tc.expect {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

func TestDependencyTargetsExtension_V8(t *testing.T) {
	dep := extensionDependency{ExtensionID: "ext-a", Version: ""}
	got := dependencyTargetsExtension(dep, "ext-b", "1.0.0")
	if got {
		t.Fatal("expected false for different extension")
	}
	got = dependencyTargetsExtension(dep, "ext-a", "1.0.0")
	if !got {
		t.Fatal("expected true for empty version constraint")
	}
}

func TestNormalizeAndValidateExtensionScope_V8(t *testing.T) {
	st, sid := "", ""
	err := normalizeAndValidateExtensionScope(&st, &sid)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	st, sid = "game", ""
	err = normalizeAndValidateExtensionScope(&st, &sid)
	if err == nil {
		t.Fatal("expected error for only type")
	}
	st, sid = "", "123"
	err = normalizeAndValidateExtensionScope(&st, &sid)
	if err == nil {
		t.Fatal("expected error for only id")
	}
	st, sid = "invalid", "123"
	err = normalizeAndValidateExtensionScope(&st, &sid)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	st, sid = "game", "123"
	err = normalizeAndValidateExtensionScope(&st, &sid)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	st, sid = "GAME", "123"
	err = normalizeAndValidateExtensionScope(&st, &sid)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if st != "game" {
		t.Fatalf("expected lowercase 'game', got %q", st)
	}
}

func TestMatchSingleClause_V8(t *testing.T) {
	cur := semVersion{1, 0, 0, 3}
	if matchSingleClause(cur, "!1.0.0") {
		t.Fatal("expected false for unknown operator")
	}
	if matchSingleClause(cur, ">invalid") {
		t.Fatal("expected false for invalid target")
	}
	if !matchSingleClause(cur, "=1.0.0") {
		t.Fatal("expected true for =1.0.0")
	}
	if !matchSingleClause(cur, ">0.9.0") {
		t.Fatal("expected true for >0.9.0")
	}
	if !matchSingleClause(cur, ">=1.0.0") {
		t.Fatal("expected true for >=1.0.0")
	}
	if !matchSingleClause(cur, "<1.0.1") {
		t.Fatal("expected true for <1.0.1")
	}
	if !matchSingleClause(cur, "<=1.0.0") {
		t.Fatal("expected true for <=1.0.0")
	}
}

func TestMatchVersionConstraint_V8(t *testing.T) {
	tests := []struct {
		current, constraint string
		want                bool
	}{
		{"1.0.0", "", true},
		{"1.0.0", "1.0.0", true},
		{"1.0.0", "2.0.0", false},
		{"1.0.0", ">=1.0.0", true},
		{"1.0.0", ">=1.0.1", false},
		{"1.0.1", ">=1.0.0", true},
		{"1.0.0", "<=1.0.0", true},
		{"1.0.1", "<=1.0.0", false},
		{"1.0.0", ">1.0.0", false},
		{"1.0.1", ">1.0.0", true},
		{"1.0.0", "<1.0.1", true},
		{"1.0.1", "<1.0.1", false},
		{"1.0.0", "=1.0.0", true},
		{"1.0.0", "1.0.0,>=1.0.0", true},
		{"1.0.0", "1.0.0,>=1.0.1", false},
		{"1.0.0", "invalid", false},
		{"invalid", "1.0.0", false},
		{"1.0.0", ",,1.0.0,", true},
	}
	for _, tc := range tests {
		t.Run(tc.current+"_"+tc.constraint, func(t *testing.T) {
			got := matchVersionConstraint(tc.current, tc.constraint)
			if got != tc.want {
				t.Errorf("matchVersionConstraint(%q, %q) = %v, want %v", tc.current, tc.constraint, got, tc.want)
			}
		})
	}
}

func TestMatchCaretConstraint_V8(t *testing.T) {
	base := semVersion{1, 2, 0, 3}
	tests := []struct {
		current semVersion
		want    bool
	}{
		{semVersion{1, 2, 0, 3}, true},
		{semVersion{1, 2, 1, 3}, true},
		{semVersion{1, 3, 0, 3}, true},  // 1.3.0 is within [1.2.0, 2.0.0)
		{semVersion{2, 0, 0, 3}, false}, // 2.0.0 is at upper bound
		{semVersion{1, 1, 0, 3}, false}, // 1.1.0 is below base
	}
	for _, tc := range tests {
		got := matchCaretConstraint(tc.current, base)
		if got != tc.want {
			t.Errorf("matchCaretConstraint(%+v, %+v) = %v, want %v", tc.current, base, got, tc.want)
		}
	}
}

func TestMatchTildeConstraint_V8(t *testing.T) {
	base := semVersion{1, 2, 0, 3}
	tests := []struct {
		current semVersion
		want    bool
	}{
		{semVersion{1, 2, 0, 3}, true},
		{semVersion{1, 2, 5, 3}, true},
		{semVersion{1, 3, 0, 3}, false},
		{semVersion{1, 1, 0, 3}, false},
	}
	for _, tc := range tests {
		got := matchTildeConstraint(tc.current, base)
		if got != tc.want {
			t.Errorf("matchTildeConstraint(%+v, %+v) = %v, want %v", tc.current, base, got, tc.want)
		}
	}
}

func TestMatchTildeConstraint_MajorOnly_V8(t *testing.T) {
	base := semVersion{1, 0, 0, 1}
	tests := []struct {
		current semVersion
		want    bool
	}{
		{semVersion{1, 0, 0, 3}, true},
		{semVersion{1, 5, 0, 3}, true},
		{semVersion{2, 0, 0, 3}, false},
	}
	for _, tc := range tests {
		got := matchTildeConstraint(tc.current, base)
		if got != tc.want {
			t.Errorf("matchTildeConstraint(%+v, %+v) = %v, want %v", tc.current, base, got, tc.want)
		}
	}
}

func TestParseSemVersion_V8(t *testing.T) {
	tests := []struct {
		input  string
		expect semVersion
		ok     bool
	}{
		{"1.2.3", semVersion{major: 1, minor: 2, patch: 3, parts: 3}, true},
		{"v1.2.3", semVersion{major: 1, minor: 2, patch: 3, parts: 3}, true},
		{"1.2", semVersion{major: 1, minor: 2, parts: 2}, true},
		{"1", semVersion{major: 1, parts: 1}, true},
		{"", semVersion{}, false},
		{"abc", semVersion{}, false},
		{"1.2.3+build", semVersion{major: 1, minor: 2, patch: 3, parts: 3}, true},
		{"1.2.3-beta", semVersion{major: 1, minor: 2, patch: 3, parts: 3}, true},
		{"1.2.3.4", semVersion{}, false},
		{" 1.2.3 ", semVersion{major: 1, minor: 2, patch: 3, parts: 3}, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := parseSemVersion(tc.input)
			if ok != tc.ok {
				t.Fatalf("ok=%v, want %v", ok, tc.ok)
			}
			if ok && got != tc.expect {
				t.Fatalf("got %+v, want %+v", got, tc.expect)
			}
		})
	}
}

func TestCompareSemVersion_V8(t *testing.T) {
	tests := []struct {
		a, b semVersion
		want int
	}{
		{semVersion{1, 0, 0, 3}, semVersion{1, 0, 0, 3}, 0},
		{semVersion{2, 0, 0, 3}, semVersion{1, 0, 0, 3}, 1},
		{semVersion{1, 0, 0, 3}, semVersion{2, 0, 0, 3}, -1},
		{semVersion{1, 2, 0, 3}, semVersion{1, 1, 0, 3}, 1},
		{semVersion{1, 1, 0, 3}, semVersion{1, 2, 0, 3}, -1},
		{semVersion{1, 1, 2, 3}, semVersion{1, 1, 1, 3}, 1},
		{semVersion{1, 1, 1, 3}, semVersion{1, 1, 2, 3}, -1},
	}
	for _, tc := range tests {
		got := compareSemVersion(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("compareSemVersion(%+v, %+v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestValidateConfigAgainstSchema_V8(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{},
		"required":   []any{},
	}
	err := validateConfigAgainstSchema(nil, schema)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	schema = map[string]any{
		"properties": map[string]any{},
		"required":   []any{"host", "port"},
	}
	err = validateConfigAgainstSchema(map[string]any{}, schema)
	if err == nil {
		t.Fatal("expected error for missing required")
	}
	schema = map[string]any{
		"properties": map[string]any{},
		"required":   []any{"host"},
	}
	err = validateConfigAgainstSchema(map[string]any{"host": "localhost"}, schema)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	schema = map[string]any{
		"properties": map[string]any{
			"host": map[string]any{"type": "string"},
			"port": map[string]any{"type": "number"},
		},
		"required": []any{"host"},
	}
	err = validateConfigAgainstSchema(map[string]any{"host": "localhost", "port": float64(8080)}, schema)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateConfigField_V8(t *testing.T) {
	rule := map[string]any{"enum": []any{"a", "b", "c"}}
	err := validateConfigField("test", "a", rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", "d", rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": ""}
	err = validateConfigField("test", "anything", rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	rule = map[string]any{"type": "string"}
	err = validateConfigField("test", "hello", rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", 123, rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": "boolean"}
	err = validateConfigField("test", true, rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", "true", rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": "number"}
	err = validateConfigField("test", float64(1.5), rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", "1.5", rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": "integer"}
	err = validateConfigField("test", int(1), rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", 1.5, rule)
	if err == nil {
		t.Fatal("expected error")
	}
	err = validateConfigField("test", float64(1), rule)
	if err != nil {
		t.Fatalf("expected nil for float64(1), got %v", err)
	}
	err = validateConfigField("test", float32(1), rule)
	if err != nil {
		t.Fatalf("expected nil for float32(1), got %v", err)
	}
	rule = map[string]any{"type": "object"}
	err = validateConfigField("test", map[string]any{"a": 1}, rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", "not an object", rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": "array"}
	err = validateConfigField("test", []any{1, 2}, rule)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	err = validateConfigField("test", "not an array", rule)
	if err == nil {
		t.Fatal("expected error")
	}
	rule = map[string]any{"type": "unknown"}
	err = validateConfigField("test", "anything", rule)
	if err != nil {
		t.Fatalf("expected nil for unknown type, got %v", err)
	}
}

func TestParseDependencies_V8(t *testing.T) {
	got := parseDependencies(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	got = parseDependencies(map[string]any{})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	manifest := map[string]any{
		"dependencies": []any{
			map[string]any{"extension_id": "ext-a", "version": ">=1.0.0"},
			map[string]any{"id": "ext-b", "required_version": "2.0.0"},
		},
	}
	got = parseDependencies(manifest)
	if len(got) != 2 {
		t.Fatalf("expected 2 deps, got %v", got)
	}
	got = parseDependencies(map[string]any{"dependencies": "not an array"})
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
	manifest = map[string]any{
		"dependencies": []any{
			map[string]any{"extension_id": ""},
		},
	}
	got = parseDependencies(manifest)
	if len(got) != 0 {
		t.Fatalf("expected empty for empty ID, got %v", got)
	}
}

func TestIsJSONNumberType_V8(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		expect bool
	}{
		{"int", int(1), true}, {"int8", int8(1), true}, {"int16", int16(1), true},
		{"int32", int32(1), true}, {"int64", int64(1), true},
		{"uint", uint(1), true}, {"uint8", uint8(1), true}, {"uint16", uint16(1), true},
		{"uint32", uint32(1), true}, {"uint64", uint64(1), true},
		{"float32", float32(1.5), true}, {"float64", float64(1.5), true},
		{"string", "1", false}, {"bool", true, false}, {"nil", nil, false},
		{"struct", struct{}{}, false},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := isJSONNumberType(c.value)
			if got != c.expect {
				t.Errorf("isJSONNumberType(%v) = %v, want %v", c.value, got, c.expect)
			}
		})
	}
}

func TestIsJSONIntegerType_V8(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		expect bool
	}{
		{"int", int(1), true},
		{"float64 int", float64(1), true},
		{"float64 non-int", float64(1.5), false},
		{"float32 int", float32(1), true},
		{"float32 non-int", float32(1.5), false},
		{"string", "1", false}, {"nil", nil, false},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got := isJSONIntegerType(c.value)
			if got != c.expect {
				t.Errorf("isJSONIntegerType(%v) = %v, want %v", c.value, got, c.expect)
			}
		})
	}
}

func TestExtractCapabilityDetailsFromBindings_V8(t *testing.T) {
	caps, details := extractCapabilityDetailsFromBindings(nil)
	if len(caps) != 0 || len(details) != 0 {
		t.Fatalf("expected empty, got caps=%v details=%v", caps, details)
	}

	// capability binding
	bindings := []model.ExtensionRuntimeBinding{
		{BindingType: "capability", BindingKey: "admin"},
	}
	caps, details = extractCapabilityDetailsFromBindings(bindings)
	if len(caps) == 0 {
		t.Fatal("expected at least 1 cap")
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if details[0].Capability != "admin" {
		t.Errorf("expected capability 'admin', got %q", details[0].Capability)
	}

	// empty capability key
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "capability", BindingKey: ""},
	}
	caps, _ = extractCapabilityDetailsFromBindings(bindings)
	if len(caps) != 0 {
		t.Fatalf("expected 0 caps for empty key, got %v", caps)
	}

	// capability with operations and permissions
	specJSON := []byte(`{"operations": ["create", "read"], "permissions": {"create": "write"}, "config_keys": ["host"]}`)
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "capability", BindingKey: "data", SpecJSON: specJSON},
	}
	_, details = extractCapabilityDetailsFromBindings(bindings)
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}
	if len(details[0].Operations) != 2 {
		t.Errorf("expected 2 operations, got %d", len(details[0].Operations))
	}
	if len(details[0].Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(details[0].Permissions))
	}
	if len(details[0].ConfigKeys) != 1 {
		t.Errorf("expected 1 config key, got %d", len(details[0].ConfigKeys))
	}

	// duplicate operations
	specJSON = []byte(`{"operations": ["create", "read", "create"]}`)
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "capability", BindingKey: "data", SpecJSON: specJSON},
	}
	_, details = extractCapabilityDetailsFromBindings(bindings)
	if len(details) != 1 || len(details[0].Operations) != 2 {
		t.Errorf("expected 2 deduplicated operations, got %v", details)
	}

	// function binding
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "function", BindingKey: "analytics:process"},
	}
	caps, _ = extractCapabilityDetailsFromBindings(bindings)
	if len(caps) == 0 {
		t.Fatal("expected at least 1 cap from function binding")
	}

	// unknown binding type
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "unknown", BindingKey: "something"},
	}
	caps, _ = extractCapabilityDetailsFromBindings(bindings)
	if len(caps) != 0 {
		t.Fatalf("expected 0 caps for unknown binding type, got %v", caps)
	}

	// empty binding
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "", BindingKey: ""},
	}
	caps, _ = extractCapabilityDetailsFromBindings(bindings)
	if len(caps) != 0 {
		t.Fatalf("expected 0 caps for empty binding, got %v", caps)
	}
}

func TestExtractPageDetailsFromBindings_V8(t *testing.T) {
	items := extractPageDetailsFromBindings(nil)
	if len(items) != 0 {
		t.Fatalf("expected empty, got %v", items)
	}

	// page binding
	specJSON := []byte(`{"title": "Dashboard", "route": "/dashboard", "icon": "dashboard", "group": "main", "required_permission": "admin:all", "order": 1}`)
	bindings := []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "dashboard", SpecJSON: specJSON},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Dashboard" {
		t.Errorf("expected title 'Dashboard', got %q", items[0].Title)
	}

	// ui binding
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "ui", BindingKey: "settings", SpecJSON: []byte(`{"title": "Settings"}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 1 {
		t.Fatalf("expected 1 item from ui binding, got %d", len(items))
	}

	// navigation binding
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "navigation", BindingKey: "nav", SpecJSON: []byte(`{"title": "Nav"}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 1 {
		t.Fatalf("expected 1 item from navigation binding, got %d", len(items))
	}

	// non-page binding type
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "capability", BindingKey: "cap"},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 0 {
		t.Fatalf("expected 0 items for non-page binding, got %v", items)
	}

	// empty binding key, use spec id
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "", SpecJSON: []byte(`{"id": "page-from-spec", "title": "From Spec"}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	// duplicate keys
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "dup", SpecJSON: []byte(`{"title": "First"}`)},
		{BindingType: "page", BindingKey: "dup", SpecJSON: []byte(`{"title": "Second"}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) != 1 {
		t.Fatalf("expected 1 item (deduplicated), got %d", len(items))
	}

	// order from float64
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "p1", SpecJSON: []byte(`{"title": "P1", "order": 5}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) == 1 && items[0].Order != 5 {
		t.Errorf("expected order 5, got %d", items[0].Order)
	}

	// empty title defaults to key
	bindings = []model.ExtensionRuntimeBinding{
		{BindingType: "page", BindingKey: "mykey", SpecJSON: []byte(`{}`)},
	}
	items = extractPageDetailsFromBindings(bindings)
	if len(items) == 1 && items[0].Title != "mykey" {
		t.Errorf("expected title 'mykey', got %q", items[0].Title)
	}
}
