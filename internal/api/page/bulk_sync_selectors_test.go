package page

import (
	"context"
	"net/http"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redriftPlayerQuery 在页面发布后重写 player.query 契约。version 与 risk
// 由用例控制：版本不变的纯 schema 漂移是 selector 同步可修复的形态；
// 版本/risk 漂移在 post-sync freshness 里仍进 Manual（发布快照未变），
// 批量同步按设计跳过。schema 采用真实分页契约形态（page+page_size
// 齐备，新增 keyword 筛选），与 bulk-republish 漂移基准一致。
func redriftPlayerQuery(t *testing.T, service *Service, version, risk string) {
	t.Helper()
	db := service.svcCtx.DB
	ctx := context.Background()
	rebuildFunctionContract(t, db, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      version,
		Risk:         risk,
		Permission:   "player:query",
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"},"keyword":{"type":"string"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"createdAt":{"type":"string"},"keyword":{"type":"string"}}}},"total":{"type":"number"}}}`,
	})
	require.NoError(t, contractsvc.NewContractService(db).RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, contractsvc.NewContractService(db).RebuildProposalsForResource(ctx, "demo-game", "development", "player"))
}

// F：契约变更批量 selector 同步——默认集合（Inbox ContractChanges 全量
// pageKey）逐页 apply：selector 拉齐 + DraftRevision+1；published 快照
// 不动（同步只写草稿，上线走 bulk-republish）。
func TestBulkSyncSelectors_SyncsSchemaDriftPages(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	const pageKey = "resource--player"
	draftBefore, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	publishedBefore, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)

	// 版本不变的纯 schema 漂移：selector 可修复、post-sync freshness 干净。
	redriftPlayerQuery(t, service, "1.0.0", "safe")

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, []string{pageKey}, resp.Synced)
	assert.Empty(t, resp.Skipped)
	assert.Empty(t, resp.Failed)

	draftAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, draftBefore.DraftRevision+1, draftAfter.DraftRevision, "apply 必须递增 DraftRevision")

	publishedAfter, err := service.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, publishedBefore.SpecJSON, publishedAfter.SpecJSON, "同步只写 draft，published 快照不动")
	assert.Equal(t, publishedBefore.Version, publishedAfter.Version)
}

// F：governance 漂移（risk 变更）不可由 selector 同步修复——dry-run 判定
// Manual 非空 → 整页 Skipped 并透传诊断，草稿不动。
func TestBulkSyncSelectors_SkipsGovernanceDrift(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	const pageKey = "resource--player"
	draftBefore, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)

	// risk 必须是合法枚举值（非法值会被 mustParseRisk 回落 safe，漂移
	// 不生效）。
	redriftPlayerQuery(t, service, "1.0.0", "high")

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{PageKeys: []string{pageKey}})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Empty(t, resp.Synced)
	assert.Empty(t, resp.Failed)
	require.Len(t, resp.Skipped, 1)
	assert.Equal(t, pageKey, resp.Skipped[0].PageKey)
	assert.Equal(t, "manual_required", resp.Skipped[0].Reason)
	foundGovernance := false
	for _, d := range resp.Skipped[0].Manual {
		if d.Code == "binding_governance_stale" {
			foundGovernance = true
		}
	}
	assert.True(t, foundGovernance, "Manual 必须透传 governance 漂移诊断，got %+v", resp.Skipped[0].Manual)

	draftAfter, err := service.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, "demo-game", "development", pageKey)
	require.NoError(t, err)
	assert.Equal(t, draftBefore.DraftRevision, draftAfter.DraftRevision, "Skipped 页面草稿不动")
}

// F：显式 pageKeys 只处理指定页面；不存在的页面进入 failed 且不中断。
func TestBulkSyncSelectors_ExplicitKeys_ReportFailures(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	redriftPlayerQuery(t, service, "1.0.0", "safe")

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{PageKeys: []string{"resource:missing", " ", "resource--player"}})
	require.NoError(t, err)
	// 空白 key 被过滤，不计数
	assert.Equal(t, 2, resp.Total)
	assert.Contains(t, resp.Synced, "resource--player")
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource:missing", resp.Failed[0]["pageKey"])
}

// F：批量同步要求 pages:edit 权限（编辑语义，非 publish）。
func TestBulkSyncSelectors_RequiresEditPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:read")
	_, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.Error(t, err)
}

// F：单页 apply 失败（sqlite 触发器 RAISE 阻断 UPDATE）只计入 failed，
// 批量入口不冒泡错误。
func TestBulkSyncSelectors_TxUpsertErrorContinues(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit", "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	redriftPlayerQuery(t, service, "1.0.0", "safe")

	require.NoError(t, service.svcCtx.DB.Exec(
		"CREATE TRIGGER bulk_sync_block_page_specs_update BEFORE UPDATE ON page_specs "+
			"BEGIN SELECT RAISE(ABORT, 'injected upsert failure bulk-sync'); END").Error)

	resp, err := service.BulkSyncSelectors(ctx, &PageBulkSyncSelectorsRequest{})
	require.NoError(t, err, "单页失败不冒泡")
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource--player", resp.Failed[0]["pageKey"])
	assert.Contains(t, resp.Failed[0]["error"], "injected upsert failure bulk-sync")
}

// 路由注册：POST /api/v1/pages/bulk-sync-selectors 必须存在（与
// bulk-republish 同级，web 客户端契约）。
func TestBulkSyncSelectorsRouteMatchesWebClientContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterDraftRoutes(engine.Group("/api/v1/pages"), nil)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/pages/bulk-sync-selectors" {
			return
		}
	}
	t.Fatal("POST /api/v1/pages/bulk-sync-selectors is not registered")
}
