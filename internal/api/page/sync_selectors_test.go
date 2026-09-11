package page

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 路由注册：POST /api/v1/pages/:pageKey/sync-selectors 必须存在（与
// regenerate 同级，web 客户端契约）。
func TestSyncSelectorsRouteMatchesWebClientContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterDraftRoutes(engine.Group("/api/v1/pages"), nil)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/pages/:pageKey/sync-selectors" {
			return
		}
	}
	t.Fatal("POST /api/v1/pages/:pageKey/sync-selectors is not registered")
}

// handler 绑定路径：URI + JSON 双绑定后进 service（缺失 scope 时按既有
// 风格返回 4xx/5xx，不 panic）。
func TestHandler_SyncSelectors_BindJSON(t *testing.T) {
	h := setupTestHandler(t)

	ctx, rec := newTestContext(http.MethodPost, "/api/v1/pages/test-page/sync-selectors",
		`{"draftRevision":1,"dryRun":true,"bindingIds":["list"]}`)
	ctx.Params = gin.Params{{Key: "pageKey", Value: "test-page"}}
	h.SyncSelectors(ctx)
	assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError}, rec.Code)
}

// ---------------------------------------------------------------------------
// sync-selectors service 层测试矩阵：权限/404/409/dry-run 不落库/apply 落
// 库（revision + PageVersion + 审计）/binding 过滤/函数缺失透传/发布链闭环。
// ---------------------------------------------------------------------------

func syncSelectorsBadRequest(err error) bool {
	var codeErr *errorx.CodeError
	return errors.As(err, &codeErr) && codeErr.Code == http.StatusBadRequest
}

func syncSelectorsConflict(err error) bool {
	var codeErr *errorx.CodeError
	return errors.As(err, &codeErr) && codeErr.Code == http.StatusConflict
}

// draftRevision 缺失 → 400。
func TestSyncSelectors_RequiresDraftRevision(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")

	_, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.True(t, syncSelectorsBadRequest(err), "expected 400, got %v", err)
}

// 页面不存在 → 404。
func TestSyncSelectors_PageNotFound(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := 1

	_, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "missing-page",
		DraftRevision: &revision,
	})
	var notFound *PageNotFoundError
	require.ErrorAs(t, err, &notFound)
}

// revision 过期 → 409（SaveDraft 同款乐观锁）。
func TestSyncSelectors_RevisionConflict(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)
	stale := revision + 100

	_, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &stale,
		DryRun:        true,
	})
	require.Error(t, err)
	assert.True(t, syncSelectorsConflict(err), "expected 409, got %v", err)
}

// 无 pages:edit 权限 → 拒绝。
func TestSyncSelectors_NoPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	revision := 1

	_, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
}

// scope 缺失 → 拒绝。
func TestSyncSelectors_NoScope(t *testing.T) {
	service, _, _ := newPageTestService(t, "pages:edit")
	revision := 1
	ctx := context.WithValue(context.Background(), "username", "page_tester")

	_, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
	})
	require.Error(t, err)
}

// dryRun 只出报告：revision/SpecJSON/PageVersion 都不落。
func TestSyncSelectors_DryRunDoesNotPersist(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	before, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	versionsBefore := syncCountPageVersions(t, service, "player.manage")

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.NoError(t, err)
	assert.False(t, resp.Applied)
	assert.True(t, resp.DryRun)
	assert.Equal(t, revision, resp.DraftRevision)

	after, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", "player.manage")
	require.NoError(t, err)
	assert.Equal(t, before.DraftRevision, after.DraftRevision)
	assert.Equal(t, string(before.SpecJSON), string(after.SpecJSON))
	assert.Equal(t, versionsBefore, syncCountPageVersions(t, service, "player.manage"))
}

// bindingIds 过滤：不匹配的 binding 不进报告。
func TestSyncSelectors_BindingFilter(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
		BindingIDs:    []string{"nonexistent-binding"},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.SyncedBindings)
}

// 函数契约被删：binding_function_missing 以 Manual 透传，不整体失败。
func TestSyncSelectors_MissingFunctionManualReport(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:read")
	revision := saveTestPageDraft(t, service, ctx)

	require.NoError(t, service.svcCtx.DB.
		Where("game_id = ? AND env = ? AND function_id = ?", "demo-game", "development", "player.query").
		Delete(&model.FunctionContract{}).Error)

	resp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       "player.manage",
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SyncedBindings)
	found := false
	for _, report := range resp.SyncedBindings {
		if report.FunctionID != "player.query" {
			continue
		}
		found = true
		require.NotEmpty(t, report.Manual)
		assert.Equal(t, "binding_function_missing", report.Manual[0].Code)
	}
	assert.True(t, found, "missing-function binding must appear in report")
}

// 发布链闭环：契约 v1 → 提案 → publish → 契约 v2（字段 rename）→ 发布被
// 阻断 → sync dryRun 报告（renamed/high）→ apply（revision+1 + PageVersion
// + 审计）→ Publish 成功 → freshness 干净（console binding_stale 409 的
// 判定源不再报 stale）。
func TestSyncSelectors_PublishChainEndToEnd(t *testing.T) {
	service, ctx, auditStore := newPageTestService(t, "pages:edit", "pages:publish")
	db := service.svcCtx.DB
	const functionID = "gift.send"
	const proposalKey = "operation:gift.send"

	// 1. 契约 v1 注册 + 提案 + 发布
	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"uid":{"type":"string"},"count":{"type":"integer"}},"required":["uid","count"]}`,
	))
	require.NoError(t, dashboardservice.NewContractService(db).RebuildProposalForFunction(ctx, "demo-game", "development", functionID))
	published, err := dashboardservice.NewProposalService(db).AcceptAndPublishProposal(ctx, "demo-game", "development", proposalKey)
	require.NoError(t, err)
	pageKey := published.PageKey
	require.NotEmpty(t, pageKey)

	// 2. 契约 v2：uid → player_id（同类型同 required）
	rebuildFunctionContract(t, db, ctx, functionID, syncGiftMeta(
		`{"type":"object","properties":{"player_id":{"type":"string"},"count":{"type":"integer"}},"required":["player_id","count"]}`,
	))

	// prev schema 必须已持久化（rename 精确推断的前提）
	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "demo-game", "development", functionID)
	require.NoError(t, err)
	assert.Contains(t, string(contract.PrevInputSchema), `"uid"`)
	assert.Contains(t, string(contract.InputSchema), `"player_id"`)

	// 3. v1 草稿发布被阻断（selector target 消失 → 发布级校验 error）
	revision := published.DraftRevision
	_, err = service.Publish(ctx, &PagePublishRequest{PageKey: pageKey, DraftRevision: &revision})
	require.Error(t, err, "stale draft must be blocked from publishing")

	// 4. dry-run：renamed(high)、remaining 无 error、不落库
	dryResp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       pageKey,
		DraftRevision: &revision,
		DryRun:        true,
	})
	require.NoError(t, err)
	assert.False(t, dryResp.Applied)
	require.Len(t, dryResp.SyncedBindings, 1)
	report := dryResp.SyncedBindings[0]
	require.True(t, report.Changed)
	var renamedEntry *spec.SelectorSyncInputEntry
	for i := range report.Input {
		if report.Input[i].Action == spec.SelectorSyncRenamed {
			renamedEntry = &report.Input[i]
		}
	}
	require.NotNil(t, renamedEntry, "expected a renamed entry, got %+v", report.Input)
	assert.Equal(t, "/player_id", renamedEntry.NewTarget)
	assert.Equal(t, spec.SelectorSyncConfidenceHigh, renamedEntry.Confidence,
		"prev schema matches the published digest; rename must be precise")
	for _, diag := range dryResp.RemainingDiagnostics {
		assert.NotEqual(t, spec.SeverityError, diag.Severity,
			"after sync no error-level diagnostics should remain: %+v", diag)
	}

	// 5. apply：revision+1、草稿 selector 指向新字段、PageVersion + 审计
	applyResp, err := service.SyncSelectors(ctx, &PageSyncSelectorsRequest{
		PageKey:       pageKey,
		DraftRevision: &revision,
	})
	require.NoError(t, err)
	assert.True(t, applyResp.Applied)
	assert.Equal(t, revision+1, applyResp.DraftRevision)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: pageKey})
	require.NoError(t, err)
	assert.Equal(t, revision+1, draft.DraftRevision)
	require.NotEmpty(t, draft.Bindings)
	require.NotNil(t, draft.Bindings[0].Selectors)
	var target string
	for _, assignment := range draft.Bindings[0].Selectors.Input.Assignments {
		if assignment.Source.Kind == spec.SourceForm && assignment.Source.Path != "" {
			target = assignment.Target
		}
	}
	assert.Equal(t, "/player_id", target, "draft selector must point at the renamed field")

	pageVersions, err := service.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	foundVersion := false
	for _, version := range pageVersions {
		if version.Version == revision+1 && version.Message == "sync selectors from latest function contracts" {
			foundVersion = true
		}
	}
	assert.True(t, foundVersion, "PageVersion with sync message must exist")

	records, _, err := auditStore.List(audit.AuditFilter{
		EventType:  []audit.AuditEventType{audit.EventPageDraftSave},
		ResourceID: pageKey,
	}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	foundAudit := false
	for _, record := range records {
		if action, ok := record.Details["action"]; ok && action == "sync_selectors" {
			foundAudit = true
		}
	}
	assert.True(t, foundAudit, "audit event with action=sync_selectors must exist")

	// 6. 发布成功（此前被阻断的同一草稿）
	newRevision := applyResp.DraftRevision
	publishResp, err := service.Publish(ctx, &PagePublishRequest{PageKey: pageKey, DraftRevision: &newRevision})
	require.NoError(t, err, "publish must succeed after selector sync")
	assert.Equal(t, newRevision, publishResp.PublishedVersion)

	// 7. 新发布快照对最新契约 freshness 干净（console binding_stale 409 的判定源）
	stale := service.bindingFreshnessForPublishedDraft(ctx, &model.PageSpec{
		GameID: "demo-game", Env: "development", PageKey: pageKey, PublishedVersion: publishResp.PublishedVersion,
	})
	assert.Empty(t, stale, "published snapshot must be fresh after sync + publish")
}

func syncGiftMeta(inputSchema string) reg.FunctionMeta {
	return reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		InputSchema:  inputSchema,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
	}
}

func syncCountPageVersions(t *testing.T, service *Service, pageKey string) int {
	t.Helper()
	versions, err := service.svcCtx.PageVersionModel.ListByScopeAndPageKey(
		t.Context(), "demo-game", "development", pageKey)
	require.NoError(t, err)
	return len(versions)
}
