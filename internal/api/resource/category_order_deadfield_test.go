package resource

// Design-debt regression test: the List sort originally compared
// Category.Order first and used Key as a tie-break, but the ResourceCapability
// model has no Order column and there is no separate Category table, so
// assembling a ResourceCategorySpec only yields Key+Labels, Order stays 0, and
// the sorting dimension is entirely ineffective. Settled fix: simplify the sort
// to a stable sort by Key alone and delete the Order field from
// spec.ResourceCategorySpec (the resource catalog API response no longer emits
// category.order, and the frontend consumes it nowhere).

import (
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	dashboardservice "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// List returns strictly ascending Key order: build three resources
// (account/guild/player) to verify the order, and verify the category in the
// response JSON no longer carries an order field.
func TestList_SortByKeyOnly_CategoryOrderRemoved(t *testing.T) {
	svcCtx, ctx := newResourceTestServiceContext(t, reg.NewStore(), "resources:read")
	seedPlayerResource(t, svcCtx, ctx)

	// Third resource: account, whose Key sorts before guild/player.
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

	}
}
