package page

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

// --- isValidPageType ---
func TestIsValidPageType_V7(t *testing.T) {
	tests := []struct {
		input spec.PageType
		want  bool
	}{
		{spec.PageTypeResource, true},
		{spec.PageTypeOperation, true},
		{spec.PageTypeTask, true},
		{spec.PageTypeReport, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isValidPageType(tt.input); got != tt.want {
			t.Errorf("isValidPageType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- validatePageShape ---
func TestValidatePageShape_V7(t *testing.T) {
	diags := validatePageShape(spec.PageSpec{Type: spec.PageTypeResource})
	hasError := false
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "expected error for resource page without resource spec")

	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeResource, Resource: &spec.ResourcePageSpec{}})
	assert.Empty(t, diags)

	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeOperation})
	hasError = false
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "expected error for operation page without operation spec")

	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeTask})
	hasError = false
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "expected error for task page without task spec")

	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeReport})
	hasError = false
	for _, d := range diags {
		if d.Severity == spec.SeverityError {
			hasError = true
			break
		}
	}
	assert.True(t, hasError, "expected error for report page without report spec")
}

// --- bindingsByID ---
func TestBindingsByID_V7(t *testing.T) {
	bindings := []spec.PageFunctionBinding{
		{ID: "a", FunctionID: "f1"},
		{ID: "", FunctionID: "f2"},
		{ID: "b", FunctionID: "f3"},
	}
	m := bindingsByID(bindings)
	assert.Len(t, m, 2)
	assert.Equal(t, "f1", m["a"].FunctionID)
	assert.Equal(t, "f3", m["b"].FunctionID)
}

// --- bindingRequiresOutputSelectors ---
func TestBindingRequiresOutputSelectors_V7(t *testing.T) {
	page := spec.PageSpec{Type: spec.PageTypeResource}
	binding := spec.PageFunctionBinding{Usage: spec.BindingUsageQuery}
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	page = spec.PageSpec{Type: spec.PageTypeTask}
	binding = spec.PageFunctionBinding{Usage: spec.BindingUsageTaskStatus}
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	page = spec.PageSpec{Type: spec.PageTypeReport}
	binding = spec.PageFunctionBinding{Usage: spec.BindingUsageReport}
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	binding = spec.PageFunctionBinding{Usage: spec.BindingUsageAction}
	assert.False(t, bindingRequiresOutputSelectors(binding, page))
}

// --- schemaHasFields ---
func TestSchemaHasFields_V7(t *testing.T) {
	assert.False(t, schemaHasFields(nil))
	assert.False(t, schemaHasFields([]byte{}))

	schema := []byte(`{"properties":{"name":{"type":"string"}}}`)
	assert.True(t, schemaHasFields(schema))

	schema = []byte(`{"required":["name"]}`)
	assert.True(t, schemaHasFields(schema))

	schema = []byte(`{"type":"object"}`)
	assert.False(t, schemaHasFields(schema))
}

// --- isValidUsage ---
func TestIsValidUsage_V7(t *testing.T) {
	validUsages := []spec.PageBindingUsage{
		spec.BindingUsageQuery,
		spec.BindingUsageDetail,
		spec.BindingUsageAction,
		spec.BindingUsageTask,
		spec.BindingUsageTaskStatus,
		spec.BindingUsageTaskEvents,
		spec.BindingUsageTaskResult,
		spec.BindingUsageTaskCancel,
		spec.BindingUsageTaskRetry,
		spec.BindingUsageReport,
	}
	for _, u := range validUsages {
		assert.True(t, isValidUsage(u), "usage %q should be valid", u)
	}
	assert.False(t, isValidUsage("invalid"))
}

// --- isValidExecutionMode ---
func TestIsValidExecutionMode_V7(t *testing.T) {
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeSync))
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeTask))
	assert.False(t, isValidExecutionMode("invalid"))
}

// --- normalizeLocaleKeys ---
func TestNormalizeLocaleKeys_V7(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{"nil", nil, nil},
		{"empty", map[string]string{}, nil},
		{"zh shorthand", map[string]string{"zh": "你好"}, map[string]string{"zh-CN": "你好"}},
		{"en shorthand", map[string]string{"en": "Hello"}, map[string]string{"en-US": "Hello"}},
		{"zh-CN", map[string]string{"zh-CN": "你好"}, map[string]string{"zh-CN": "你好"}},
		{"en-US", map[string]string{"en-US": "Hello"}, map[string]string{"en-US": "Hello"}},
		{"underscore", map[string]string{"zh_cn": "你好"}, map[string]string{"zh-CN": "你好"}},
		{"mixed valid/invalid", map[string]string{"zh-CN": "你好", "fr": "Bonjour"}, map[string]string{"zh-CN": "你好", "fr": "Bonjour"}},
		{"all empty values", map[string]string{"zh": "  "}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLocaleKeys(tt.input)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			for k, v := range tt.want {
				assert.Equal(t, v, got[k])
			}
		})
	}
}

// --- hasDefaultLocale ---
func TestHasDefaultLocale_V7(t *testing.T) {
	assert.False(t, hasDefaultLocale(nil))
	assert.False(t, hasDefaultLocale(spec.LocalizedText{}))
	assert.True(t, hasDefaultLocale(spec.LocalizedText{"zh-CN": "你好"}))
	assert.False(t, hasDefaultLocale(spec.LocalizedText{"en-US": "Hello"}))
}

// --- localizedTextEqual ---
func TestLocalizedTextEqual_V7(t *testing.T) {
	left := map[string]string{"zh-CN": "你好", "en-US": "Hello"}
	right := map[string]string{"zh-cn": "你好", "en-us": "Hello"}
	assert.True(t, localizedTextEqual(left, right))

	right2 := map[string]string{"zh-CN": "你好", "en-US": "Hello2"}
	assert.False(t, localizedTextEqual(left, right2))

	assert.False(t, localizedTextEqual(left, map[string]string{"zh-CN": "你好"}))
}

// --- countErrors ---
func TestCountErrors_V7(t *testing.T) {
	diags := []spec.Diagnostic{
		{Severity: spec.SeverityError},
		{Severity: spec.SeverityWarning},
		{Severity: spec.SeverityError},
	}
	assert.Equal(t, 2, countErrors(diags))
	assert.Equal(t, 0, countErrors(nil))
}

// --- diagnosticsToDetails ---
func TestDiagnosticsToDetails_V7(t *testing.T) {
	diags := []spec.Diagnostic{
		{Field: "type", Message: "invalid"},
		{Code: "unknown_code", Message: "something"},
	}
	details := diagnosticsToDetails(diags)
	assert.Equal(t, "invalid", details["type"])
	assert.Equal(t, "something", details["unknown_code"])
}

// --- diagnostic helper ---
func TestDiagnostic_V7(t *testing.T) {
	d := diagnostic("test_code", spec.SeverityError, "test message", "test.field")
	assert.Equal(t, "test_code", d.Code)
	assert.Equal(t, spec.SeverityError, d.Severity)
	assert.Equal(t, "test message", d.Message)
	assert.Equal(t, "test.field", d.Field)
}

// --- diagnosticsFromJSON ---
func TestDiagnosticsFromJSON_V7(t *testing.T) {
	assert.Empty(t, diagnosticsFromJSON(nil))
	assert.Empty(t, diagnosticsFromJSON([]byte{}))

	diags := diagnosticsFromJSON([]byte("not json"))
	assert.Len(t, diags, 1)
	assert.Equal(t, "proposal_diagnostics_invalid", diags[0].Code)

	diags = diagnosticsFromJSON([]byte(`[{"code":"test","severity":"error","message":"m","field":"f"}]`))
	assert.Len(t, diags, 1)
	assert.Equal(t, "test", diags[0].Code)
}

// --- localizedTextFromJSONMap ---
func TestLocalizedTextFromJSONMap_V7(t *testing.T) {
	assert.Nil(t, localizedTextFromJSONMap(nil))
	assert.Nil(t, localizedTextFromJSONMap(map[string]interface{}{}))

	result := localizedTextFromJSONMap(map[string]interface{}{
		"zh-CN": "你好",
		"en-US": "Hello",
		"empty": "",
	})
	assert.NotNil(t, result)
	assert.Equal(t, "你好", result["zh-CN"])
	assert.Equal(t, "Hello", result["en-US"])
	_, hasEmpty := result["empty"]
	assert.False(t, hasEmpty, "empty string should not be included")

	// Non-string values should be ignored
	result = localizedTextFromJSONMap(map[string]interface{}{
		"num": 123,
	})
	assert.Nil(t, result, "non-string values should return nil")
}

// --- approvalPolicyFromJSONMap ---
func TestApprovalPolicyFromJSONMap_V7(t *testing.T) {
	policy := approvalPolicyFromJSONMap(nil)
	assert.False(t, policy.Required)
	assert.Empty(t, policy.PolicyKey)

	policy = approvalPolicyFromJSONMap(map[string]interface{}{
		"required":  true,
		"policyKey": "admin",
	})
	assert.True(t, policy.Required)
	assert.Equal(t, "admin", policy.PolicyKey)

	// Test snake_case fallback
	policy = approvalPolicyFromJSONMap(map[string]interface{}{
		"policy_key": "snaked",
	})
	assert.Equal(t, "snaked", policy.PolicyKey)
}

// --- requireScope ---
func TestRequireScope_V7(t *testing.T) {
	ctx := context.Background()
	_, _, err := requireScope(ctx)
	assert.Error(t, err, "expected error for empty scope")

	ctx = svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1"})
	_, _, err = requireScope(ctx)
	assert.Error(t, err, "expected error when env is missing")

	ctx = svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game1", Env: "prod"})
	gameID, env, err := requireScope(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "game1", gameID)
	assert.Equal(t, "prod", env)
}

// --- pageSpecFromModel ---
func TestPageSpecFromModel_V7(t *testing.T) {
	_, err := pageSpecFromModel(nil)
	assert.Error(t, err, "nil model should return error")

	_, err = pageSpecFromModel(&model.PageSpec{})
	assert.Error(t, err, "empty spec should return error")

	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: "not json"})
	assert.Error(t, err, "invalid JSON should return error")

	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: `{"type":"resource"}`})
	assert.Error(t, err, "empty pageKey should return error")

	ps, err := pageSpecFromModel(&model.PageSpec{SpecJSON: `{"pageKey":"test","type":"resource"}`})
	assert.NoError(t, err)
	assert.Equal(t, "test", ps.PageKey)
}

// --- pageSpecFromPublishedModel ---
func TestPageSpecFromPublishedModel_V7(t *testing.T) {
	_, err := pageSpecFromPublishedModel(model.PublishedPageSpec{})
	assert.Error(t, err, "empty spec should return error")

	_, err = pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: "not json"})
	assert.Error(t, err, "invalid JSON should return error")

	_, err = pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: `{"type":"resource"}`})
	assert.Error(t, err, "empty pageKey should return error")

	ps, err := pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: `{"pageKey":"test","type":"resource"}`})
	assert.NoError(t, err)
	assert.Equal(t, "test", ps.PageKey)
}

// --- pageSpecFromProposalModel ---
func TestPageSpecFromProposalModel_V7(t *testing.T) {
	_, err := pageSpecFromProposalModel(nil)
	assert.Error(t, err, "nil should return error")

	_, err = pageSpecFromProposalModel(&model.PageProposal{})
	assert.Error(t, err, "empty PageSpec should return error")

	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte("not json")})
	assert.Error(t, err, "invalid JSON should return error")

	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte(`{"type":"resource"}`)})
	assert.Error(t, err, "empty pageKey should return error")

	ps, err := pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte(`{"pageKey":"test","type":"resource","title":{"zh-CN":"你好"}}`)})
	assert.NoError(t, err)
	assert.Equal(t, "test", ps.PageKey)
}

// --- ErrPageNotFound ---
func TestErrPageNotFound_V7(t *testing.T) {
	err := ErrPageNotFound("mypage")
	assert.NotNil(t, err)
	assert.Equal(t, "page not found: mypage", err.Error())
}

// --- digestRaw ---
func TestDigestRaw_V7(t *testing.T) {
	assert.Empty(t, digestRaw(nil))
	assert.Empty(t, digestRaw([]byte{}))
	got := digestRaw([]byte("hello"))
	assert.NotEmpty(t, got)
	assert.Len(t, got, 64, "expected 64-char hex")
}

// --- buildBindingContracts ---
func TestBuildBindingContracts_V7(t *testing.T) {
	functions := map[string]spec.FunctionSpec{
		"f1": {Version: "v1", Enabled: true},
	}

	bindings := []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "f1", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
	}
	snaps, err := buildBindingContracts(bindings, functions)
	assert.NoError(t, err)
	assert.Len(t, snaps, 1)
	assert.Equal(t, "v1", snaps[0].FunctionVersion)

	// Missing function
	bindings = []spec.PageFunctionBinding{
		{ID: "b1", FunctionID: "missing", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
	}
	_, err = buildBindingContracts(bindings, functions)
	assert.Error(t, err, "expected error for missing function")
}

// --- marshalPageSpec ---
func TestMarshalPageSpec_V7(t *testing.T) {
	page := spec.PageSpec{
		PageKey:     "  test  ",
		ResourceKey: "  res  ",
		Icon:        "  icon  ",
		Title:       spec.LocalizedText{"zh-CN": "你好"},
		Description: spec.LocalizedText{"en-US": "Hello"},
		Category:    spec.PageCategorySpec{Key: "  cat  ", Labels: spec.LocalizedText{"zh-CN": "分类"}},
		Bindings: []spec.PageFunctionBinding{
			{ID: "  b1  ", FunctionID: "  f1  "},
		},
	}
	result, err := marshalPageSpec(page)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

// --- parsePublishedPageForFreshness ---
func TestParsePublishedPageForFreshness_V7(t *testing.T) {
	pageSpec, contracts := parsePublishedPageForFreshness(model.PublishedPageSpec{})
	assert.Empty(t, pageSpec.PageKey)
	assert.Empty(t, contracts)
}
