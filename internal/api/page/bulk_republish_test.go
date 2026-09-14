package page

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// driftFunctionContract 在页面发布后重写 player.query 契约（版本与 schema
// 双变），使已发布页面的绑定评估进入 stale。schema 采用真实分页契约形态
// （page+page_size 齐备，新增 keyword 筛选）——只带 page 无 page_size 的
// 契约会触发生成器预存边界（分页 selector 指向 /current 但分页未启用，
// 页面不可发布），与本功能无关。
func driftFunctionContract(t *testing.T, service *Service) {
	t.Helper()
	db := service.svcCtx.DB
	ctx := context.Background()
	rebuildFunctionContract(t, db, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "2.0.0",
		Risk:         "safe",
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

// F：契约变更一键重新发布——自动发现 stale 已发布页，逐页重生成+发布；
// 重新发布后队列清空（幂等，二次执行无目标）。
func TestBulkRepublish_RepublishesStalePublishedPages(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	// 未漂移时：无 stale 已发布页，无目标
	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)

	driftFunctionContract(t, service)

	resp, err = service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Contains(t, resp.Published, "resource--player")
	assert.Empty(t, resp.Failed)

	// 已拉齐到最新契约：二次执行无 stale 目标
	resp2, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp2.Total)
	assert.Empty(t, resp2.Published)
}

// F：显式 pageKeys 只处理指定页面；不存在的页面进入 failed 且不中断。
func TestBulkRepublish_ExplicitKeys_ReportFailures(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:publish")
	seedResourceProposal(t, service)

	pub, err := service.BulkPublish(ctx, &PageBulkRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, pub.Published)

	driftFunctionContract(t, service)

	resp, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{PageKeys: []string{"resource:missing", " ", "resource--player"}})
	require.NoError(t, err)
	// 空白 key 被过滤，不计数
	assert.Equal(t, 2, resp.Total)
	assert.Contains(t, resp.Published, "resource--player")
	require.Len(t, resp.Failed, 1)
	assert.Equal(t, "resource:missing", resp.Failed[0]["pageKey"])
}

// F：批量重新发布要求 pages:publish 权限。
func TestBulkRepublish_RequiresPublishPermission(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	_, err := service.BulkRepublish(ctx, &PageBulkRepublishRequest{})
	require.Error(t, err)
}
