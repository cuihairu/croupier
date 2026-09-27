package resourcecatalog

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Categories 聚合接口回归（OPEN-ISSUES #14）：过滤下拉必须拿到全量 distinct
// 分类，不能由前端从（已按分类过滤的）列表推导——否则选一项后选项塌缩成一项。
func TestService_Categories(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	capModel := model.NewResourceCapabilityModel(db)
	seed := []model.ResourceCapability{
		// 无审核分类：按 resource key 前缀推导 → player / mail
		{GameID: "demo-game", Env: "development", ResourceKey: "player"},
		{GameID: "demo-game", Env: "development", ResourceKey: "player.item"},
		{GameID: "demo-game", Env: "development", ResourceKey: "mail.template"},
		// 审核分类优先于前缀推导
		{GameID: "demo-game", Env: "development", ResourceKey: "legacy.thing", CategoryKey: "archived"},
		// 其他 scope 不应计入
		{GameID: "other-game", Env: "development", ResourceKey: "order"},
	}
	for i := range seed {
		require.NoError(t, capModel.UpsertCapability(ctx, &seed[i]))
	}

	resp, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)

	require.Len(t, resp.Items, 3)
	assert.Equal(t, []CategoryOption{
		{CategoryKey: "archived", Count: 1},
		{CategoryKey: "mail", Count: 1},
		{CategoryKey: "player", Count: 2},
	}, resp.Items)

	// scope 隔离：另一 scope 只有 order 一项
	other, err := service.Categories(ctx, "other-game", "development")
	require.NoError(t, err)
	require.Len(t, other.Items, 1)
	assert.Equal(t, "order", other.Items[0].CategoryKey)
}

// 缓存行为：命中期内新增行不重查（返回旧集合），invalidate 后重查拿到新分类。
// 这同时锁住「写路径主动失效」语义——UpdateSemantics 改分类后下拉不会展示
// 30s 的陈旧选项。
func TestService_Categories_CacheAndInvalidation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	capModel := model.NewResourceCapabilityModel(db)
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
	}))

	first, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	require.Len(t, first.Items, 1)

	// 缓存命中期内的新增行不可见（TTL 未到）
	require.NoError(t, capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "demo-game", Env: "development", ResourceKey: "mail.template",
	}))
	cached, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, cached.Items, 1, "cache hit should serve the stale set until invalidated")

	// 失效后重查 → 全量
	service.invalidateCategoryCache("demo-game", "development")
	fresh, err := service.Categories(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Len(t, fresh.Items, 2)
}
