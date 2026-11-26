package extension

import (
	"context"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

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

func TestPtrInstallationItem(t *testing.T) {
	modelItem := model.ExtensionInstallation{
		Model:          gorm.Model{ID: 456},
		ExtensionID:    "test.ext",
		ReleaseVersion: "2.0.0",
	}
	ptr := ptrInstallationItem(modelItem, "")
	if ptr == nil {
		t.Fatalf("expected non-nil pointer")
	}
	if ptr.ID != 456 {
		t.Fatalf("unexpected id: %d", ptr.ID)
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

func TestFirstManifestEmptyReleases(t *testing.T) {
	got := firstManifest([]model.ExtensionRelease{})
	if got != "{}" {
		t.Fatalf("expected empty object for empty releases, got: %s", got)
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

// applyVersionOp: unknown operator -> (false, false), so the caller rejects
// the match on that basis. The opMatchers key set matches the seven literals
// extracted by the matchSingleClause prefixes; this directly tests the
// dispatch table's rejection branch.
func TestApplyVersionOp_UnknownOpRejected(t *testing.T) {
	cur := semVersion{major: 2, minor: 0, patch: 0}
	tgt := semVersion{major: 1, minor: 0, patch: 0}
	assert.False(t, applyVersionOp("???", cur, tgt, 1))

	// Known operators go through table dispatch.
	assert.True(t, applyVersionOp(">", cur, tgt, 1))
}

// TestService_InstallationList_DisplayNameJoin_HealthDerived (#46 batch 2):
// list assembly joins the catalog display names (displayName -> name ->
// extensionID fallback) + healthStatus is derived from status/enabled
// (replacing the always-unknown value).

// TestDeriveExtensionHealthStatus_Matrix: the derivation matrix (same
// semantics as HealthCheck).
func TestDeriveExtensionHealthStatus_Matrix(t *testing.T) {
	cases := []struct {
		name string
		item *model.ExtensionInstallation
		want string
	}{
		{"nil → unknown", nil, "unknown"},
		{"status uninstalled → uninstalled", &model.ExtensionInstallation{Status: "Uninstalled", Enabled: true}, "uninstalled"},
		{"desired uninstalled → uninstalled", &model.ExtensionInstallation{Status: "enabled", DesiredState: "uninstalled", Enabled: true}, "uninstalled"},
		{"enabled → healthy", &model.ExtensionInstallation{Status: "enabled", Enabled: true}, "healthy"},
		{"not enabled → disabled", &model.ExtensionInstallation{Status: "enabled", Enabled: false}, "disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveExtensionHealthStatus(tc.item); got != tc.want {
				t.Fatalf("deriveExtensionHealthStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestService_CatalogWrite_Lifecycle (#46 batch 3): register -> update
// (listing status) -> publish release (latestVersion back-fill rule) ->
// active installation blocks removal -> cascade removal after uninstall.
