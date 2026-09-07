package page

// 覆盖目标（缝隙注入类分支，缝隙默认值与生产行为完全一致）：
//  1. service.go SaveDraft/RegenerateDraft/Publish/Rollback 的 CurrentUsername
//     失败分支：requirePage* 先经 RequireAnyPermission→LoadCurrentAdmin→
//     CurrentUsername，无 username 时在权限检查处即失败，后续调用恒成功。
//  2. service.go Publish 的 specJSON/contractsJSON marshal 失败分支：
//     pageSpec 经 pageSpecFromModel 反序列化得到（RawMessage 必然合法），
//     纯结构体序列化恒成功。
//  3. service.go RegenerateDraft 的 buildPageSpecJSON/pageSpecFromModel 失败
//     分支：p.SpecJSON 刚被 applyPageSpecToModel 覆盖为合法产物。
//  4. service.go validatePublishBindingSelectors 的 warnings 循环：
//     spec.ValidateSelector/ValidateOutputAssignments 仅 addError，
//     result.Warnings 恒为空。
//  5. handler.go Versions/VersionDetail 的 *PageNotFoundError 分支：
//     service.Versions 忽略 Find 错误、VersionDetail 仅返回 errorx.NotFound，
//     均不产生 *PageNotFoundError。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withCurrentUsernameError(t *testing.T, injected error) {
	t.Helper()
	orig := currentUsername
	currentUsername = func(context.Context) (string, error) { return "", injected }
	t.Cleanup(func() { currentUsername = orig })
}

func TestGapfixSaveDraft_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	injected := errors.New("injected username failure")
	withCurrentUsernameError(t, injected)

	rev := 0
	resp, err := service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &rev,
		Type:          spec.PageTypeOperation,
		Title:         map[string]string{"zh-CN": "页面"},
		Category:      spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
		Operation:     testOperationPageSpec(),
		Bindings:      testPageBindings(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "SaveDraft() error = %v, want injected", err)
}

func TestGapfixRegenerateDraft_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	injected := errors.New("injected username failure")
	withCurrentUsernameError(t, injected)

	resp, err := service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(1)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "RegenerateDraft() error = %v, want injected", err)
}

func TestGapfixPublish_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	injected := errors.New("injected username failure")
	withCurrentUsernameError(t, injected)

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(1)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "Publish() error = %v, want injected", err)
}

func TestGapfixRollback_CurrentUsernameError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:rollback")
	injected := errors.New("injected username failure")
	withCurrentUsernameError(t, injected)

	expected := 1
	resp, err := service.Rollback(ctx, &PageRollbackRequest{
		PageKey: "player.manage", VersionID: "1", ExpectedDraftRevision: &expected,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "Rollback() error = %v, want injected", err)
}

// Publish：pageSpec 序列化（第 1 次 marshal）失败。
func TestGapfixPublish_PageSpecMarshalError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)

	injected := errors.New("injected pagespec marshal failure")
	orig := pageJSONMarshal
	pageJSONMarshal = func(v interface{}) ([]byte, error) { return nil, injected }
	t.Cleanup(func() { pageJSONMarshal = orig })

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "Publish() error = %v, want injected", err)
}

// Publish：contracts 序列化（第 2 次 marshal）失败。
func TestGapfixPublish_ContractsMarshalError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish", "pages:edit")
	rev := saveTestPageDraft(t, service, ctx)

	injected := errors.New("injected contracts marshal failure")
	orig := pageJSONMarshal
	calls := 0
	pageJSONMarshal = func(v interface{}) ([]byte, error) {
		calls++
		if calls == 1 {
			return orig(v)
		}
		return nil, injected
	}
	t.Cleanup(func() { pageJSONMarshal = orig })

	resp, err := service.Publish(ctx, &PagePublishRequest{PageKey: "player.manage", DraftRevision: intPtrFinal(rev)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "Publish() error = %v, want injected", err)
}

// gapfixRegenerateEnv 准备一份可再生的草稿（proposal + page 记录），返回其 pageKey。
func gapfixRegenerateEnv(t *testing.T, service *Service, ctx context.Context) string {
	t.Helper()
	seedResourceProposal(t, service)

	const (
		gameID = "demo-game"
		env    = "development"
		key    = "resource:player"
	)
	proposal, err := model.NewPageProposalModel(service.svcCtx.DB).FindByScopeAndKey(ctx, gameID, env, key)
	require.NoError(t, err)
	var replacement struct {
		PageKey string `json:"pageKey"`
	}
	require.NoError(t, json.Unmarshal(proposal.PageSpec, &replacement))
	require.NotEmpty(t, replacement.PageKey)

	require.NoError(t, service.svcCtx.DB.Create(&model.PageSpec{
		GameID:          gameID,
		Env:             env,
		PageKey:         replacement.PageKey,
		CategoryKey:     "player",
		SpecJSON:        string(proposal.PageSpec),
		Status:          "draft",
		DraftRevision:   1,
		BaseProposalKey: key,
	}).Error)
	return replacement.PageKey
}

func TestGapfixRegenerateDraft_BuildSpecJSONError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	pageKey := gapfixRegenerateEnv(t, service, ctx)

	injected := errors.New("injected buildSpecJSON failure")
	orig := buildPageSpecJSONFn
	buildPageSpecJSONFn = func(*model.PageSpec) (string, error) { return "", injected }
	t.Cleanup(func() { buildPageSpecJSONFn = orig })

	resp, err := service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: pageKey, DraftRevision: intPtrFinal(1)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "RegenerateDraft() error = %v, want injected", err)
}

func TestGapfixRegenerateDraft_PageSpecFromModelError(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	pageKey := gapfixRegenerateEnv(t, service, ctx)

	injected := errors.New("injected pageSpecFromModel failure")
	orig := pageSpecFromModelFn
	pageSpecFromModelFn = func(*model.PageSpec) (spec.PageSpec, error) {
		return spec.PageSpec{}, injected
	}
	t.Cleanup(func() { pageSpecFromModelFn = orig })

	resp, err := service.RegenerateDraft(ctx, &PageRegenerateRequest{PageKey: pageKey, DraftRevision: intPtrFinal(1)})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, injected), "RegenerateDraft() error = %v, want injected", err)
}

func assertWarningDiagnostic(t *testing.T, diags []spec.Diagnostic, code, field string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code && d.Field == field {
			require.Equal(t, spec.SeverityWarning, d.Severity, "diagnostic %s must be a warning", code)
			return
		}
	}
	t.Fatalf("expected warning diagnostic %s at %s, got %#v", code, field, diags)
}

// input selector warnings：Field 非空拼接子字段、Field 为空保持基础路径。
func TestGapfixValidatePublishBindingSelectors_InputWarnings(t *testing.T) {
	orig := validateSelectorFn
	validateSelectorFn = func(spec.SelectorAST, spec.JSONSchema, spec.SelectorContext) spec.SelectorValidationResult {
		return spec.SelectorValidationResult{
			Valid: true,
			Warnings: []spec.SelectorWarning{
				{Field: "extra", Code: "gapfix_w1", Message: "warning with field"},
				{Field: "", Code: "gapfix_w2", Message: "warning without field"},
			},
		}
	}
	t.Cleanup(func() { validateSelectorFn = orig })

	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{
		Enabled:     true,
		InputSchema: spec.JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
	}
	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{},
			Output: []spec.OutputAssignment{{
				StateKey: "sel",
				Source:   "/total",
				Shape:    spec.OutputShapeScalar,
			}},
		},
		Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}

	diags := validatePublishBindingSelectors("b", binding, fn, page)
	assertWarningDiagnostic(t, diags, "binding_selector_gapfix_w1", "b.selectors.input.extra")
	assertWarningDiagnostic(t, diags, "binding_selector_gapfix_w2", "b.selectors.input")
}

// output selector warnings：Field 非空拼接子字段、Field 为空保持基础路径。
func TestGapfixValidatePublishBindingSelectors_OutputWarnings(t *testing.T) {
	orig := validateOutputAssignmentsFn
	validateOutputAssignmentsFn = func([]spec.OutputAssignment, spec.JSONSchema) spec.SelectorValidationResult {
		return spec.SelectorValidationResult{
			Valid: true,
			Warnings: []spec.SelectorWarning{
				{Field: "sel2", Code: "gapfix_w3", Message: "warning with field"},
				{Field: "", Code: "gapfix_w4", Message: "warning without field"},
			},
		}
	}
	t.Cleanup(func() { validateOutputAssignmentsFn = orig })

	page := spec.PageSpec{Type: spec.PageTypeResource}
	fn := spec.FunctionSpec{
		Enabled:      true,
		InputSchema:  spec.JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}}}`),
		OutputSchema: spec.JSONSchema(`{"type":"object","properties":{"total":{"type":"number"}}}`),
	}
	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "f1",
		Usage:      spec.BindingUsageQuery,
		Selectors: &spec.BindingSelectors{
			Output: []spec.OutputAssignment{{
				StateKey: "sel",
				Source:   "/total",
				Shape:    spec.OutputShapeScalar,
			}},
		},
		Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}

	diags := validatePublishBindingSelectors("b", binding, fn, page)
	assertWarningDiagnostic(t, diags, "binding_selector_gapfix_w3", "b.selectors.output.sel2")
	assertWarningDiagnostic(t, diags, "binding_selector_gapfix_w4", "b.selectors.output")
}

// handler Versions：service 返回 *PageNotFoundError → 404。
func TestGapfixVersionsHandler_NotFoundError(t *testing.T) {
	handler := setupTestHandler(t)

	orig := serviceVersionsFn
	serviceVersionsFn = func(*Service, context.Context, *PageVersionsRequest) (*PageVersionsResponse, error) {
		return nil, ErrPageNotFound("missing-page")
	}
	t.Cleanup(func() { serviceVersionsFn = orig })

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages/missing-page/versions", "")
	ctx.Params = gin.Params{{Key: "pageKey", Value: "missing-page"}}
	handler.Versions(ctx)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "page not found: missing-page")
}

// handler VersionDetail：service 返回 *PageNotFoundError → 404。
func TestGapfixVersionDetailHandler_NotFoundError(t *testing.T) {
	handler := setupTestHandler(t)

	orig := serviceVersionDetailFn
	serviceVersionDetailFn = func(*Service, context.Context, *PageVersionDetailRequest) (*PageVersionDetailResponse, error) {
		return nil, ErrPageNotFound("missing-page")
	}
	t.Cleanup(func() { serviceVersionDetailFn = orig })

	ctx, rec := newTestContext(http.MethodGet, "/api/v1/pages/missing-page/versions/1", "")
	ctx.Params = gin.Params{
		{Key: "pageKey", Value: "missing-page"},
		{Key: "versionId", Value: "1"},
	}
	handler.VersionDetail(ctx)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "page not found: missing-page")
}
