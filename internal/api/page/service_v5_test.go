package page

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// normalizeLocaleKeys – edge cases not covered by existing tests
// ---------------------------------------------------------------------------

func TestNormalizeLocaleKeysV5_Variants(t *testing.T) {
	// "zh" should normalize to "zh-CN"
	out := normalizeLocaleKeys(map[string]string{"zh": "你好"})
	assert.Equal(t, "你好", out["zh-CN"])

	// "zh_CN" (underscore) should normalize to "zh-CN"
	out = normalizeLocaleKeys(map[string]string{"zh_CN": "下划线"})
	assert.Equal(t, "下划线", out["zh-CN"])

	// "en" should normalize to "en-US"
	out = normalizeLocaleKeys(map[string]string{"en": "Hello"})
	assert.Equal(t, "Hello", out["en-US"])

	// "en_US" should normalize to "en-US"
	out = normalizeLocaleKeys(map[string]string{"en_US": "Hello US"})
	assert.Equal(t, "Hello US", out["en-US"])

	// whitespace values should be trimmed and empty values dropped
	out = normalizeLocaleKeys(map[string]string{"ja": "  ", "de": " Hallo "})
	assert.Equal(t, "Hallo", out["de"])
	_, hasJA := out["ja"]
	assert.False(t, hasJA)

	// all whitespace values => nil
	out = normalizeLocaleKeys(map[string]string{"a": "  ", "b": " "})
	assert.Nil(t, out)
}

// ---------------------------------------------------------------------------
// localizedTextEqual – more edge cases
// ---------------------------------------------------------------------------

func TestLocalizedTextEqualV5_EdgeCases(t *testing.T) {
	// both nil
	assert.True(t, localizedTextEqual(nil, nil))

	// one nil one empty
	assert.True(t, localizedTextEqual(nil, map[string]string{}))

	// different lengths
	left := map[string]string{"zh-CN": "a"}
	right := map[string]string{"zh-CN": "a", "en-US": "b"}
	assert.False(t, localizedTextEqual(left, right))

	// same keys different values
	left2 := map[string]string{"zh-CN": "a"}
	right2 := map[string]string{"zh-CN": "b"}
	assert.False(t, localizedTextEqual(left2, right2))

	// case normalization for keys
	left3 := map[string]string{"zh_cn": "ok"}
	right3 := map[string]string{"zh-CN": "ok"}
	assert.True(t, localizedTextEqual(left3, right3))
}

// ---------------------------------------------------------------------------
// countErrors
// ---------------------------------------------------------------------------

func TestCountErrorsV5(t *testing.T) {
	assert.Equal(t, 0, countErrors(nil))
	assert.Equal(t, 0, countErrors([]spec.Diagnostic{}))
	assert.Equal(t, 1, countErrors([]spec.Diagnostic{
		{Severity: spec.SeverityError},
	}))
	assert.Equal(t, 2, countErrors([]spec.Diagnostic{
		{Severity: spec.SeverityError},
		{Severity: spec.SeverityWarning},
		{Severity: spec.SeverityError},
	}))
}

// ---------------------------------------------------------------------------
// diagnosticsToDetails
// ---------------------------------------------------------------------------

func TestDiagnosticsToDetailsV5(t *testing.T) {
	diags := []spec.Diagnostic{
		{Code: "c1", Field: "f1", Message: "msg1"},
		{Code: "c2", Field: "", Message: "msg2"},
	}
	result := diagnosticsToDetails(diags)
	assert.Equal(t, "msg1", result["f1"])
	assert.Equal(t, "msg2", result["c2"])
}

// ---------------------------------------------------------------------------
// diagnosticsFromJSON – invalid JSON
// ---------------------------------------------------------------------------

func TestDiagnosticsFromJSONV5_InvalidJSON(t *testing.T) {
	result := diagnosticsFromJSON([]byte(`{not json}`))
	require.Len(t, result, 1)
	assert.Equal(t, "proposal_diagnostics_invalid", result[0].Code)

	// empty
	assert.Nil(t, diagnosticsFromJSON(nil))
	assert.Nil(t, diagnosticsFromJSON([]byte{}))

	// valid
	result = diagnosticsFromJSON([]byte(`[{"code":"x","severity":"warning","message":"m"}]`))
	require.Len(t, result, 1)
	assert.Equal(t, "x", result[0].Code)
}

// ---------------------------------------------------------------------------
// hasDefaultLocale
// ---------------------------------------------------------------------------

func TestHasDefaultLocaleV5(t *testing.T) {
	assert.False(t, hasDefaultLocale(nil))
	assert.False(t, hasDefaultLocale(map[string]string{}))
	assert.False(t, hasDefaultLocale(map[string]string{"zh-CN": "  "}))
	assert.True(t, hasDefaultLocale(map[string]string{"zh-CN": "ok"}))
	assert.False(t, hasDefaultLocale(map[string]string{"en-US": "ok"}))
}

// ---------------------------------------------------------------------------
// localizedTextFromJSONMap
// ---------------------------------------------------------------------------

func TestLocalizedTextFromJSONMapV5(t *testing.T) {
	assert.Nil(t, localizedTextFromJSONMap(nil))
	assert.Nil(t, localizedTextFromJSONMap(map[string]interface{}{}))

	// whitespace-only value should be excluded
	result := localizedTextFromJSONMap(map[string]interface{}{
		"zh-CN": "  ",
		"en-US": "Hello",
	})
	_, hasZH := result["zh-CN"]
	assert.False(t, hasZH)
	assert.Equal(t, "Hello", result["en-US"])

	// non-string values: 123 is not a string, so it's excluded;
	// but non-empty string values are included
	result2 := localizedTextFromJSONMap(map[string]interface{}{
		"zh-CN": "ok",
		"num":   123,
		"empty": "",
	})
	assert.Equal(t, "ok", result2["zh-CN"])
	_, hasNum := result2["num"]
	assert.False(t, hasNum)
	_, hasEmpty := result2["empty"]
	assert.False(t, hasEmpty)
}

// ---------------------------------------------------------------------------
// approvalPolicyFromJSONMap
// ---------------------------------------------------------------------------

func TestApprovalPolicyFromJSONMapV5(t *testing.T) {
	// nil
	p := approvalPolicyFromJSONMap(nil)
	assert.False(t, p.Required)
	assert.Equal(t, "", p.PolicyKey)

	// with policy_key (snake_case)
	p = approvalPolicyFromJSONMap(map[string]interface{}{
		"required":   true,
		"policy_key": "approval-flow",
	})
	assert.True(t, p.Required)
	assert.Equal(t, "approval-flow", p.PolicyKey)

	// with policyKey (camelCase)
	p = approvalPolicyFromJSONMap(map[string]interface{}{
		"required":   true,
		"policyKey":  "camel-flow",
		"policy_key": "should-not-be-used",
	})
	assert.Equal(t, "camel-flow", p.PolicyKey)

	// whitespace policy key
	p = approvalPolicyFromJSONMap(map[string]interface{}{
		"policyKey": "  trimmed  ",
	})
	assert.Equal(t, "trimmed", p.PolicyKey)
}

// ---------------------------------------------------------------------------
// pageSpecFromModel
// ---------------------------------------------------------------------------

func TestPageSpecFromModelV5(t *testing.T) {
	// nil model
	_, err := pageSpecFromModel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft is required")

	// empty specJSON
	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")

	// specJSON with empty pageKey
	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: `{"type":"operation"}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")

	// invalid JSON
	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: `{bad}`})
	require.Error(t, err)

	// valid
	ps, err := pageSpecFromModel(&model.PageSpec{
		SpecJSON: `{"pageKey":"test","type":"operation","category":{"key":"c"}}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "test", ps.PageKey)
}

// ---------------------------------------------------------------------------
// pageSpecFromPublishedModel
// ---------------------------------------------------------------------------

func TestPageSpecFromPublishedModelV5(t *testing.T) {
	// empty specJSON
	_, err := pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "published page does not contain")

	// invalid JSON
	_, err = pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: `{bad}`})
	require.Error(t, err)

	// empty pageKey
	_, err = pageSpecFromPublishedModel(model.PublishedPageSpec{SpecJSON: `{"type":"operation"}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")

	// valid
	ps, err := pageSpecFromPublishedModel(model.PublishedPageSpec{
		SpecJSON: `{"pageKey":"pub-test","type":"resource"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "pub-test", ps.PageKey)
}

// ---------------------------------------------------------------------------
// pageSpecFromProposalModel
// ---------------------------------------------------------------------------

func TestPageSpecFromProposalModelV5(t *testing.T) {
	// nil proposal
	_, err := pageSpecFromProposalModel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proposal is required")

	// empty PageSpec bytes
	_, err = pageSpecFromProposalModel(&model.PageProposal{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proposal does not contain")

	// invalid JSON
	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte(`{bad}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")

	// empty pageKey
	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte(`{"type":"resource"}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")

	// valid
	ps, err := pageSpecFromProposalModel(&model.PageProposal{
		PageSpec: []byte(`{"pageKey":"prop-test","type":"resource","resourceKey":"player","category":{"key":"c","labels":{"zh-CN":"分类"}}}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "prop-test", ps.PageKey)
}

// ---------------------------------------------------------------------------
// applyPageSpecToModel
// ---------------------------------------------------------------------------

func TestApplyPageSpecToModelV5(t *testing.T) {
	// nil target
	err := applyPageSpecToModel(nil, spec.PageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// empty category key
	m := &model.PageSpec{}
	err = applyPageSpecToModel(m, spec.PageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.key is required")

	// valid
	err = applyPageSpecToModel(m, spec.PageSpec{
		PageKey:     "test",
		Type:        "resource",
		ResourceKey: "player",
		Category:    spec.PageCategorySpec{Key: "c", Labels: spec.LocalizedText{"zh-CN": "分类"}},
		Title:       spec.LocalizedText{"zh-CN": "测试"},
	})
	require.NoError(t, err)
	assert.Equal(t, "test", m.PageKey)
	assert.Equal(t, "resource", m.Type)
}

// ---------------------------------------------------------------------------
// marshalPageSpec
// ---------------------------------------------------------------------------

func TestMarshalPageSpecV5(t *testing.T) {
	// verify whitespace trimming in output
	raw, err := marshalPageSpec(spec.PageSpec{
		PageKey:     "  trimmed  ",
		ResourceKey: "  rp  ",
		Icon:        "  icon  ",
		Title:       spec.LocalizedText{"zh-CN": "  title  "},
		Description: spec.LocalizedText{"zh-CN": "  desc  "},
		Category: spec.PageCategorySpec{
			Key:    "  cat  ",
			Labels: spec.LocalizedText{"zh-CN": "  labels  "},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "  b1  ", FunctionID: "  f1  "},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, raw, `"pageKey":"trimmed"`)
	assert.Contains(t, raw, `"resourceKey":"rp"`)
	assert.Contains(t, raw, `"icon":"icon"`)
}

// ---------------------------------------------------------------------------
// bindingsByID
// ---------------------------------------------------------------------------

func TestBindingsByIDV5_EmptyBindings(t *testing.T) {
	result := bindingsByID(nil)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// isValidUsage – all valid usages
// ---------------------------------------------------------------------------

func TestIsValidUsageV5_All(t *testing.T) {
	assert.True(t, isValidUsage(spec.BindingUsageReport))
	assert.False(t, isValidUsage("unknown"))
}

// ---------------------------------------------------------------------------
// isValidExecutionMode
// ---------------------------------------------------------------------------

func TestIsValidExecutionModeV5(t *testing.T) {
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeSync))
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeTask))
	assert.False(t, isValidExecutionMode(""))
	assert.False(t, isValidExecutionMode("async"))
}

// ---------------------------------------------------------------------------
// bindingRequiresOutputSelectors – more cases
// ---------------------------------------------------------------------------

func TestBindingRequiresOutputSelectorsV5(t *testing.T) {
	// detail usage on resource
	assert.False(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageDetail},
		spec.PageSpec{Type: spec.PageTypeResource},
	))

	// action on operation
	assert.False(t, bindingRequiresOutputSelectors(
		spec.PageFunctionBinding{Usage: spec.BindingUsageAction},
		spec.PageSpec{Type: spec.PageTypeOperation},
	))
}

// ---------------------------------------------------------------------------
// PageNotFoundError
// ---------------------------------------------------------------------------

func TestErrPageNotFoundV5(t *testing.T) {
	err := ErrPageNotFound("missing")
	require.Error(t, err)
	var notFound *PageNotFoundError
	assert.ErrorAs(t, err, &notFound)
	assert.Equal(t, "missing", notFound.Key)
	assert.Contains(t, notFound.Error(), "page not found: missing")
}

// ---------------------------------------------------------------------------
// pagePublishSource – nil cases
// ---------------------------------------------------------------------------

func TestPagePublishSourceV5_NilCases(t *testing.T) {
	svc := &Service{}
	// nil page
	result := svc.pagePublishSource(nil, "g", "e", nil)
	assert.Empty(t, result.BaseProposalKey)
}

func TestPagePublishSourceV5_AllBranches(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")

	// page with empty BaseProposalKey
	result := service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{
		PageKey: "test",
	})
	assert.Empty(t, result.BaseProposalKey)

	// page with non-existent proposal (no proposal in DB)
	result = service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{
		PageKey:         "test",
		BaseProposalKey: "nonexistent-proposal",
	})
	// The proposal model returns error, so BaseProposalKey stays
	assert.Equal(t, "nonexistent-proposal", result.BaseProposalKey)
}

func TestPagePublishSourceV5_NilService(t *testing.T) {
	svc := (*Service)(nil)
	result := svc.pagePublishSource(nil, "g", "e", &model.PageSpec{})
	assert.Empty(t, result.BaseProposalKey)
}
