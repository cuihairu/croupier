package page

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────
// ListDrafts
// ──────────────────────────────────────────────────────

func TestServiceListDraftsRequiresPageReadPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t) // no permissions
	_, err := service.ListDrafts(ctx, &PageDraftListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看页面")
}

func TestServiceListDraftsReturnsAllDrafts(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.ListDrafts(ctx, &PageDraftListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player.manage", resp.Items[0].PageKey)
	assert.Equal(t, revision, resp.Items[0].DraftRevision)
}

func TestServiceListDraftsFiltersByStatus(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.NoError(t, err)

	resp, err := service.ListDrafts(ctx, &PageDraftListRequest{Status: "draft"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	resp, err = service.ListDrafts(ctx, &PageDraftListRequest{Status: "published"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

func TestServiceListDraftsFiltersByResourceKey(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	resp, err := service.ListDrafts(ctx, &PageDraftListRequest{ResourceKey: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	resp, err = service.ListDrafts(ctx, &PageDraftListRequest{ResourceKey: "player"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

// ──────────────────────────────────────────────────────
// Validate
// ──────────────────────────────────────────────────────

func TestServiceValidateRequiresPageReadPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	_, err := service.Validate(ctx, &PageValidateRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看页面")
}

func TestServiceValidateReturnsValidForCorrectDraft(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	resp, err := service.Validate(ctx, &PageValidateRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.True(t, resp.Valid)
	assert.Empty(t, errorDiagnostics(resp.Diagnostics))
}

func TestServiceValidateRejectsMissingPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	_, err := service.Validate(ctx, &PageValidateRequest{PageKey: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

// ──────────────────────────────────────────────────────
// Preview
// ──────────────────────────────────────────────────────

func TestServicePreviewRequiresPageReadPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	_, err := service.Preview(ctx, &PagePreviewRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看页面")
}

func TestServicePreviewReturnsPageForValidDraft(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	resp, err := service.Preview(ctx, &PagePreviewRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "player.manage", resp.Page.PageKey)
	assert.Equal(t, spec.PageTypeOperation, resp.Page.Type)
}

func TestServicePreviewRejectsMissingPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	_, err := service.Preview(ctx, &PagePreviewRequest{PageKey: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

// ──────────────────────────────────────────────────────
// VersionDetail
// ──────────────────────────────────────────────────────

func TestServiceVersionDetailRequiresPageReadPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	_, err := service.VersionDetail(ctx, &PageVersionDetailRequest{PageKey: "player.manage", VersionID: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看页面")
}

func TestServiceVersionDetailReturnsVersion(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.VersionDetail(ctx, &PageVersionDetailRequest{
		PageKey:   "player.manage",
		VersionID: "1",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Version)
	assert.Equal(t, revision, resp.Version)
	assert.Equal(t, "save draft", resp.Message)
}

func TestServiceVersionDetailRejectsNotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	_, err := service.VersionDetail(ctx, &PageVersionDetailRequest{
		PageKey:   "player.manage",
		VersionID: "999",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page version not found")
}

// ──────────────────────────────────────────────────────
// Versions
// ──────────────────────────────────────────────────────

func TestServiceVersionsRequiresPageReadPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t)
	_, err := service.Versions(ctx, &PageVersionsRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看页面")
}

func TestServiceVersionsReturnsVersions(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	resp, err := service.Versions(ctx, &PageVersionsRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, 1, resp.Items[0].Version)
	assert.Equal(t, "draft", resp.Items[0].Status)
	assert.True(t, resp.Items[0].IsCurrentDraft)
	assert.False(t, resp.Items[0].IsCurrentPublished)
	assert.Equal(t, 1, resp.CurrentDraftRevision)
}

// ──────────────────────────────────────────────────────
// Rollback success path
// ──────────────────────────────────────────────────────

func TestServiceRollbackRequiresPageRollbackPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read") // no rollback permission
	revision := saveTestPageDraft(t, service, ctx)

	_, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey:               "player.manage",
		VersionID:             "1",
		ExpectedDraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权回滚页面")
}

func TestServiceRollbackSuccess(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:rollback", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey:               "player.manage",
		VersionID:             "1",
		ExpectedDraftRevision: &revision,
	})
	require.NoError(t, err)
	assert.Equal(t, revision+1, resp.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, resp.DraftRevision, draft.DraftRevision)
}

func TestServiceRollbackRejectsVersionNotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:rollback", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	_, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey:               "player.manage",
		VersionID:             "999",
		ExpectedDraftRevision: &revision,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page version not found")
}

func TestServiceRollbackRejectsNilExpectedDraftRevision(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:rollback", "pages:read")
	saveTestPageDraft(t, service, ctx)

	_, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey:   "player.manage",
		VersionID: "1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expectedDraftRevision is required")
}

// ──────────────────────────────────────────────────────
// SaveDraft additional branches
// ──────────────────────────────────────────────────────

func TestServiceSaveDraftRejectsMissingPageKey(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "  ", // blank pageKey
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试分类"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestServiceSaveDraftRejectsInvalidPageType(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "test.page",
		DraftRevision: &revision,
		Type:          "invalid_type",
		Title:         map[string]string{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试分类"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type must be resource, operation, task, or report")
}

func TestServiceSaveDraftRejectsMissingTitleLocale(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "test.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"en-US": "Test"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试分类"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title must include zh-CN locale")
}

func TestServiceSaveDraftRejectsMissingCategoryLabelsLocale(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "test.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"en-US": "Test Category"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.labels must include zh-CN locale")
}

func TestServiceSaveDraftRejectsNilDraftRevision(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey: "test.page",
		Type:    spec.PageTypeOperation,
		Title:   map[string]string{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试分类"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "draftRevision is required")
}

func TestServiceSaveDraftCreatesNewPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "new.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeResource,
		ResourceKey:   "test",
		Title:         map[string]string{"zh-CN": "新页面"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试"},
		},
		Resource: &spec.ResourcePageSpec{},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "new.page"})
	require.NoError(t, err)
	assert.Equal(t, "new.page", draft.PageKey)
	assert.Equal(t, spec.PageTypeResource, draft.Type)
}

func TestServiceSaveDraftCreatesTaskPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "task.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeTask,
		ResourceKey:   "task",
		Title:         map[string]string{"zh-CN": "任务页面"},
		Category: spec.PageCategorySpec{
			Key:    "task",
			Labels: spec.LocalizedText{"zh-CN": "任务"},
		},
		Task: &spec.TaskPageSpec{},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.DraftRevision)
}

func TestServiceSaveDraftCreatesReportPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 0

	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "report.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeReport,
		ResourceKey:   "report",
		Title:         map[string]string{"zh-CN": "报表页面"},
		Category: spec.PageCategorySpec{
			Key:    "report",
			Labels: spec.LocalizedText{"zh-CN": "报表"},
		},
		Report: &spec.ReportPageSpec{},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.DraftRevision)
}

func TestServiceSaveDraftUpdatesExistingPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := saveTestPageDraft(t, service, ctx)

	updated, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理（已更新）"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)
	assert.Equal(t, revision+1, updated.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "玩家管理（已更新）", draft.Title["zh-CN"])
}

// ──────────────────────────────────────────────────────
// SaveDraft conflict on new page with non-zero revision
// ──────────────────────────────────────────────────────

func TestServiceSaveDraftRejectsConflictOnNewPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	revision := 1 // non-zero for non-existing page

	_, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "conflict.page",
		DraftRevision: &revision,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "冲突页面"},
		Category: spec.PageCategorySpec{
			Key:    "conflict",
			Labels: spec.LocalizedText{"zh-CN": "冲突"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试")
}

// ──────────────────────────────────────────────────────
// Unpublish
// ──────────────────────────────────────────────────────

func TestServiceUnpublishRequiresPublishPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	_, err := service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权发布页面")
}

func TestServiceUnpublishRejectsMissingPage(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	_, err := service.Unpublish(ctx, &PageUnpublishRequest{PageKey: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

// ──────────────────────────────────────────────────────
// Pure helper function tests
// ──────────────────────────────────────────────────────

func TestIsValidPageTypeV2(t *testing.T) {
	assert.True(t, isValidPageType(spec.PageTypeResource))
	assert.True(t, isValidPageType(spec.PageTypeOperation))
	assert.True(t, isValidPageType(spec.PageTypeTask))
	assert.True(t, isValidPageType(spec.PageTypeReport))
	assert.False(t, isValidPageType("invalid"))
	assert.False(t, isValidPageType(""))
}

func TestIsValidUsageV2(t *testing.T) {
	assert.True(t, isValidUsage(spec.BindingUsageQuery))
	assert.True(t, isValidUsage(spec.BindingUsageDetail))
	assert.True(t, isValidUsage(spec.BindingUsageAction))
	assert.True(t, isValidUsage(spec.BindingUsageTask))
	assert.True(t, isValidUsage(spec.BindingUsageTaskStatus))
	assert.True(t, isValidUsage(spec.BindingUsageTaskEvents))
	assert.True(t, isValidUsage(spec.BindingUsageTaskResult))
	assert.True(t, isValidUsage(spec.BindingUsageTaskCancel))
	assert.True(t, isValidUsage(spec.BindingUsageTaskRetry))
	assert.True(t, isValidUsage(spec.BindingUsageReport))
	assert.False(t, isValidUsage("invalid"))
}

func TestIsValidExecutionModeV2(t *testing.T) {
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeSync))
	assert.True(t, isValidExecutionMode(spec.PageExecutionModeTask))
	assert.False(t, isValidExecutionMode("invalid"))
	assert.False(t, isValidExecutionMode(""))
}

func TestSchemaHasFieldsV2(t *testing.T) {
	assert.False(t, schemaHasFields(nil))
	assert.False(t, schemaHasFields([]byte("")))
	assert.False(t, schemaHasFields([]byte(`{"type":"object"}`)))
	assert.False(t, schemaHasFields([]byte(`{"type":"object","properties":{}}`)))
	assert.True(t, schemaHasFields([]byte(`{"type":"object","properties":{"name":{"type":"string"}}}`)))
	assert.True(t, schemaHasFields([]byte(`{"type":"object","required":["name"]}`)))
	assert.True(t, schemaHasFields([]byte(`{invalid json}`)))
}

func TestNormalizeLocaleKeysV2(t *testing.T) {
	// nil input
	assert.Nil(t, normalizeLocaleKeys(nil))
	// empty input
	assert.Nil(t, normalizeLocaleKeys(map[string]string{}))
	// zh-CN normalization
	result := normalizeLocaleKeys(map[string]string{"zh-CN": "测试", "en-US": "Test"})
	assert.Equal(t, "测试", result["zh-CN"])
	assert.Equal(t, "Test", result["en-US"])
	// zh alias
	result = normalizeLocaleKeys(map[string]string{"zh": "测试"})
	assert.Equal(t, "测试", result["zh-CN"])
	// en alias
	result = normalizeLocaleKeys(map[string]string{"en": "Test"})
	assert.Equal(t, "Test", result["en-US"])
	// empty values filtered
	result = normalizeLocaleKeys(map[string]string{"zh-CN": "  "})
	assert.Nil(t, result)
}

func TestDiagnosticsToDetailsV2(t *testing.T) {
	diags := []spec.Diagnostic{
		{Code: "err1", Field: "field1", Message: "msg1"},
		{Code: "err2", Field: "", Message: "msg2"},
	}
	details := diagnosticsToDetails(diags)
	assert.Equal(t, "msg1", details["field1"])
	assert.Equal(t, "msg2", details["err2"])
}

func TestDiagnosticsFromJSONV2(t *testing.T) {
	// nil
	assert.Nil(t, diagnosticsFromJSON(nil))
	// empty
	assert.Nil(t, diagnosticsFromJSON([]byte("")))
	// invalid JSON
	result := diagnosticsFromJSON([]byte(`{invalid}`))
	require.Len(t, result, 1)
	assert.Equal(t, "proposal_diagnostics_invalid", result[0].Code)
	// valid JSON
	result = diagnosticsFromJSON([]byte(`[{"code":"test","severity":"warning","message":"test msg","field":"f"}]`))
	require.Len(t, result, 1)
	assert.Equal(t, "test", result[0].Code)
}

func TestLocalizedTextEqualV2(t *testing.T) {
	assert.True(t, localizedTextEqual(nil, nil))
	assert.True(t, localizedTextEqual(
		map[string]string{"zh-CN": "测试"},
		map[string]string{"zh-CN": "测试"},
	))
	assert.False(t, localizedTextEqual(
		map[string]string{"zh-CN": "测试1"},
		map[string]string{"zh-CN": "测试2"},
	))
	assert.False(t, localizedTextEqual(
		map[string]string{"zh-CN": "测试"},
		map[string]string{"zh-CN": "测试", "en-US": "Test"},
	))
}

func TestLocalizedTextFromJSONMapV2(t *testing.T) {
	assert.Nil(t, localizedTextFromJSONMap(nil))
	assert.Nil(t, localizedTextFromJSONMap(map[string]interface{}{}))
	// all empty
	assert.Nil(t, localizedTextFromJSONMap(map[string]interface{}{"zh-CN": "  "}))
	// valid
	result := localizedTextFromJSONMap(map[string]interface{}{
		"zh-CN": "测试",
		"en-US": "Test",
	})
	assert.Equal(t, "测试", result["zh-CN"])
	assert.Equal(t, "Test", result["en-US"])
	// non-string filtered
	result = localizedTextFromJSONMap(map[string]interface{}{
		"zh-CN": 123,
	})
	assert.Nil(t, result)
}

func TestApprovalPolicyFromJSONMapV2(t *testing.T) {
	// nil
	result := approvalPolicyFromJSONMap(nil)
	assert.False(t, result.Required)
	// empty
	result = approvalPolicyFromJSONMap(map[string]interface{}{})
	assert.False(t, result.Required)
	// with values
	result = approvalPolicyFromJSONMap(map[string]interface{}{
		"required":  true,
		"policyKey": "two-person",
	})
	assert.True(t, result.Required)
	assert.Equal(t, "two-person", result.PolicyKey)
	// underscore variant
	result = approvalPolicyFromJSONMap(map[string]interface{}{
		"required":   true,
		"policy_key": "admin",
	})
	assert.Equal(t, "admin", result.PolicyKey)
}

func TestErrPageNotFoundV2(t *testing.T) {
	err := ErrPageNotFound("test-page")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found: test-page")

	var notFound *PageNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
	assert.Equal(t, "test-page", notFound.Key)
}

func TestBindingsByIDV2(t *testing.T) {
	bindings := []spec.PageFunctionBinding{
		{ID: "a", FunctionID: "fn.a"},
		{ID: "", FunctionID: "fn.empty"},
		{ID: "b", FunctionID: "fn.b"},
	}
	result := bindingsByID(bindings)
	assert.Len(t, result, 2)
	assert.Equal(t, "fn.a", result["a"].FunctionID)
	assert.Equal(t, "fn.b", result["b"].FunctionID)
	_, ok := result[""]
	assert.False(t, ok)
}

func TestBindingRequiresOutputSelectorsV2(t *testing.T) {
	// query binding on resource page
	binding := spec.PageFunctionBinding{Usage: spec.BindingUsageQuery}
	page := spec.PageSpec{Type: spec.PageTypeResource}
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	// query binding on operation page
	page.Type = spec.PageTypeOperation
	assert.False(t, bindingRequiresOutputSelectors(binding, page))

	// report binding on report page
	binding.Usage = spec.BindingUsageReport
	page.Type = spec.PageTypeReport
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	// task status binding on task page
	binding.Usage = spec.BindingUsageTaskStatus
	page.Type = spec.PageTypeTask
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	// task events binding on task page
	binding.Usage = spec.BindingUsageTaskEvents
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	// task result binding on task page
	binding.Usage = spec.BindingUsageTaskResult
	assert.True(t, bindingRequiresOutputSelectors(binding, page))

	// action binding (default)
	binding.Usage = spec.BindingUsageAction
	assert.False(t, bindingRequiresOutputSelectors(binding, page))
}

func TestCountErrorsV2(t *testing.T) {
	diags := []spec.Diagnostic{
		{Severity: spec.SeverityError},
		{Severity: spec.SeverityWarning},
		{Severity: spec.SeverityError},
		{Severity: spec.SeverityInfo},
	}
	assert.Equal(t, 2, countErrors(diags))
}

func TestHasDefaultLocaleV2(t *testing.T) {
	assert.False(t, hasDefaultLocale(nil))
	assert.False(t, hasDefaultLocale(spec.LocalizedText{}))
	assert.False(t, hasDefaultLocale(spec.LocalizedText{"zh-CN": "  "}))
	assert.True(t, hasDefaultLocale(spec.LocalizedText{"zh-CN": "测试"}))
}

func TestDigestRawV2(t *testing.T) {
	assert.Equal(t, "", digestRaw(nil))
	assert.Equal(t, "", digestRaw([]byte{}))
	assert.NotEmpty(t, digestRaw([]byte("hello")))
}

func TestDiagnosticV2(t *testing.T) {
	d := diagnostic("code", spec.SeverityError, "message", "field")
	assert.Equal(t, "code", d.Code)
	assert.Equal(t, spec.SeverityError, d.Severity)
	assert.Equal(t, "message", d.Message)
	assert.Equal(t, "field", d.Field)
}

func TestRequireScopeV2(t *testing.T) {
	// missing gameID
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{})
	_, _, err := requireScope(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")

	// missing env
	ctx = svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g"})
	_, _, err = requireScope(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Env is required")

	// valid
	ctx = svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g", Env: "e"})
	gameID, env, err := requireScope(ctx)
	require.NoError(t, err)
	assert.Equal(t, "g", gameID)
	assert.Equal(t, "e", env)
}

func TestPageSpecFromModelV2(t *testing.T) {
	// nil
	_, err := pageSpecFromModel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page draft is required")

	// empty specJSON
	_, err = pageSpecFromModel(&model.PageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")

	// invalid specJSON
	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: "{invalid}"})
	require.Error(t, err)

	// valid but empty pageKey
	_, err = pageSpecFromModel(&model.PageSpec{SpecJSON: `{"type":"operation"}`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestPageSpecFromPublishedModelV2(t *testing.T) {
	// empty specJSON
	_, err := pageSpecFromPublishedModel(model.PublishedPageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical PageSpec")

	// valid but empty pageKey
	_, err = pageSpecFromPublishedModel(model.PublishedPageSpec{
		SpecJSON: `{"type":"operation"}`,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestPageSpecFromProposalModelV2(t *testing.T) {
	// nil proposal
	_, err := pageSpecFromProposalModel(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proposal is required")

	// empty PageSpec
	_, err = pageSpecFromProposalModel(&model.PageProposal{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain canonical PageSpec")

	// invalid JSON
	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte("{invalid}")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")

	// empty pageKey
	_, err = pageSpecFromProposalModel(&model.PageProposal{PageSpec: []byte(`{"type":"operation"}`)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pageKey is required")
}

func TestApplyPageSpecToModelV2(t *testing.T) {
	// nil target
	err := applyPageSpecToModel(nil, spec.PageSpec{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// missing category key
	p := &model.PageSpec{}
	err = applyPageSpecToModel(p, spec.PageSpec{
		PageKey: "test",
		Type:    "operation",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "category.key is required")
}

func TestValidatePageShapeV2(t *testing.T) {
	// resource without resource spec
	diags := validatePageShape(spec.PageSpec{Type: spec.PageTypeResource})
	assert.Len(t, diags, 1)
	assert.Equal(t, "page_shape_missing", diags[0].Code)

	// operation without operation spec
	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeOperation})
	assert.Len(t, diags, 1)
	assert.Equal(t, "page_shape_missing", diags[0].Code)

	// task without task spec
	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeTask})
	assert.Len(t, diags, 1)
	assert.Equal(t, "page_shape_missing", diags[0].Code)

	// report without report spec
	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeReport})
	assert.Len(t, diags, 1)
	assert.Equal(t, "page_shape_missing", diags[0].Code)

	// valid operation
	diags = validatePageShape(spec.PageSpec{Type: spec.PageTypeOperation, Operation: &spec.OperationPageSpec{}})
	assert.Empty(t, diags)
}

func TestMarshalPageSpecV2(t *testing.T) {
	ps := spec.PageSpec{
		PageKey:     "  test  ",
		ResourceKey: "  res  ",
		Icon:        "  icon  ",
		Title:       spec.LocalizedText{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "  cat  ",
			Labels: spec.LocalizedText{"zh-CN": "分类"},
		},
		Bindings: []spec.PageFunctionBinding{
			{ID: "  b1  ", FunctionID: "  fn1  "},
		},
	}
	result, err := marshalPageSpec(ps)
	require.NoError(t, err)
	assert.Contains(t, result, `"pageKey":"test"`)
	assert.Contains(t, result, `"resourceKey":"res"`)
}

func TestPageNotFoundErrorV2(t *testing.T) {
	err := &PageNotFoundError{Key: "my-page"}
	assert.Equal(t, "page not found: my-page", err.Error())
}

func TestPagePublishSourceV2(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")

	// nil page
	source := service.pagePublishSource(ctx, "demo-game", "development", nil)
	assert.Empty(t, source.BaseProposalKey)

	// page without proposal key
	source = service.pagePublishSource(ctx, "demo-game", "development", &model.PageSpec{})
	assert.Empty(t, source.FunctionDigest)
}

func TestBindingFreshnessForPublishedDraftV2(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	_, err := service.Publish(ctx, &PagePublishRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.NoError(t, err)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	// After publishing, the draft should show the published version
	assert.Equal(t, revision, draft.PublishedVersion)
	assert.Equal(t, "published", draft.Status)
}

func TestValidateBindingV2(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")

	functions := service.normalizedFunctions(ctx)
	page := spec.PageSpec{Type: spec.PageTypeOperation}

	// empty binding ID
	diags := validateBinding("bindings[0]", spec.PageFunctionBinding{}, functions, page, false)
	assert.True(t, len(diags) > 0)
	found := false
	for _, d := range diags {
		if d.Code == "binding_id_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// missing function ID
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{ID: "test"}, functions, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_function_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// function not found
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{ID: "test", FunctionID: "nonexistent"}, functions, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_function_not_found" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// disabled function
	_FUNCTIONS := map[string]spec.FunctionSpec{
		"disabled.fn": {ID: "disabled.fn", Enabled: false},
	}
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{ID: "test", FunctionID: "disabled.fn"}, _FUNCTIONS, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_function_disabled" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// invalid usage
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "player.query",
		Usage:      "invalid",
	}, functions, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_usage_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// invalid execution mode
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "player.query",
		Usage:      spec.BindingUsageAction,
		Execution:  spec.PageBindingExecution{Mode: "invalid"},
	}, functions, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_execution_mode_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// task binding with non-task mode
	diags = validateBinding("bindings[0]", spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "player.query",
		Usage:      spec.BindingUsageTask,
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}, functions, page, false)
	found = false
	for _, d := range diags {
		if d.Code == "binding_task_mode_mismatch" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestValidatePageSpecV2(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")

	// invalid page type
	diags := service.validatePageSpec(ctx, spec.PageSpec{Type: "invalid"}, false)
	found := false
	for _, d := range diags {
		if d.Code == "page_type_invalid" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// missing title
	diags = service.validatePageSpec(ctx, spec.PageSpec{Type: spec.PageTypeOperation}, false)
	found = false
	for _, d := range diags {
		if d.Code == "localized_text_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// missing category key
	diags = service.validatePageSpec(ctx, spec.PageSpec{
		Type:  spec.PageTypeOperation,
		Title: spec.LocalizedText{"zh-CN": "测试"},
	}, false)
	found = false
	for _, d := range diags {
		if d.Code == "category_key_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// missing bindings
	diags = service.validatePageSpec(ctx, spec.PageSpec{
		Type:  spec.PageTypeOperation,
		Title: spec.LocalizedText{"zh-CN": "测试"},
		Category: spec.PageCategorySpec{
			Key:    "test",
			Labels: spec.LocalizedText{"zh-CN": "测试"},
		},
	}, false)
	found = false
	for _, d := range diags {
		if d.Code == "bindings_missing" {
			found = true
			break
		}
	}
	assert.True(t, found)
}
