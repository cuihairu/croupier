package resource

// 设计债回归测试：List 排序原先先比 Category.Order 再按 Key tie-break，但
// ResourceCapability 模型无 Order 列、无独立 Category 表，装配
// ResourceCategorySpec 时只有 Key+Labels，Order 恒 0，排序维度彻底无效。
// 已定案修法：排序简化为纯 Key 稳定排序，spec.ResourceCategorySpec 删除
// Order 字段（resource catalog API 响应不再输出 category.order，前端零消费）。

import (
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// List 返回严格按 Key 升序：构造三个资源（account/guild/player）验证顺序，
// 并验证响应 JSON 中 category 不再携带 order 字段。
func TestList_SortByKeyOnly_CategoryOrderRemoved(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	// 第三个资源：account，Key 排在 guild/player 之前。
	contractService := dashboardservice.NewContractService(svcCtx.DB)
	require.NoError(t, contractService.RebuildContractFromFunctionMeta(ctx, "demo-game", "development", "sdk", dashboardservice.FunctionMetaInput{
		ID:         "account.get",
		Version:    "1.0.0",
		Enabled:    true,
		Summary:    "Get account",
		Resource:   "account",
		Risk:       "safe",
		Operation:  "get",
		Capability: "item_query",
		Execution:  "sync",
		Permission: "account:get",
	}))
	require.NoError(t, contractService.RebuildResourceCapability(ctx, "demo-game", "development", "account"))

	resp, err := NewService(svcCtx).List(ctx, &ResourceListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)

	keys := make([]string, 0, len(resp.Items))
	for _, item := range resp.Items {
		keys = append(keys, item.Key)
	}
	assert.Equal(t, []string{"account", "guild", "player"}, keys, "List 必须严格按 Key 升序")

	for _, item := range resp.Items {
		assert.NotEmpty(t, item.Category.Key)
		assert.NotEmpty(t, item.Category.Labels)
	}
}
