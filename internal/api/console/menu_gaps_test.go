package console

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateMenuFromMenuItems 的错误分支：菜单树可读但页面表缺失 → 整体报错。
func TestServiceMenuPageSpecTableMissing(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_specs"))
	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// published 快照表缺失 → 整体报错（菜单树与 draft 表均可读）。
func TestServiceMenuPublishedTableMissing(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("published_page_specs"))
	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// DB 故障注入：菜单树读取失败 → Menu 报错（不 panic）。
func TestServiceMenuDBClosed(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	sqlDB, err := service.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// seedConsolePublishedIconPage 带图标的已发布页（icon 回落分支专用）。
func seedConsolePublishedIconPage(svcCtx *svc.ServiceContext, ctx context.Context, pageKey, icon string, order int) error {
	scope := svc.GameScopeFromContext(ctx)
	page := spec.PageSpec{
		PageKey:     pageKey,
		Type:        spec.PageTypeOperation,
		ResourceKey: "beta",
		Title:       spec.LocalizedText{"zh-CN": pageKey},
		Category:    spec.PageCategorySpec{Key: "beta"},
		Order:       order,
		Icon:        icon,
		Operation:   testConsoleOperationPageSpec(),
		Bindings: []spec.PageFunctionBinding{
			{
				ID:         "player.query",
				FunctionID: "player.query",
				Usage:      spec.BindingUsageQuery,
				Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
			},
		},
	}
	specJSON, err := json.Marshal(page)
	if err != nil {
		return err
	}
	return svcCtx.PublishedPageSpecModel.Create(ctx, &model.PublishedPageSpec{
		GameID:                scope.GameID,
		Env:                   scope.Env,
		PageKey:               pageKey,
		Version:               1,
		SpecJSON:              string(specJSON),
		BindingContractsJSON:  "[]",
		RendererSchemaVersion: "page-spec:1",
		FunctionDigest:        "function-digest-1",
		SemanticsDigest:       "semantics-digest-1",
		GeneratorVersion:      "page-generator:test",
		Active:                true,
		PublishedAt:           time.Now(),
		PublishedBy:           "console_tester",
	})
}

// 组装细节：同序菜单按本地化标题/key 排序、菜单无 icon 时回落组内页面
// 图标、en-US 文案回落、挂载但未发布的页面不进树（menuID nil/未命中分支）。
func TestServiceMenuAssemblyOrderingAndIconFallback(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")

	// 两个同序菜单：标题决定顺序
	_, err := seedConsoleMenu(service.svcCtx, ctx, "beta", spec.LocalizedText{"zh-CN": "乙组"}, 10, "")
	require.NoError(t, err)
	_, err = seedConsoleMenu(service.svcCtx, ctx, "alpha", spec.LocalizedText{"zh-CN": "甲组"}, 10, "")
	require.NoError(t, err)
	// 第三个同序同标题菜单：key 决定顺序（beta < gamma）
	_, err = seedConsoleMenu(service.svcCtx, ctx, "gamma", spec.LocalizedText{"zh-CN": "乙组", "en-US": "Beta"}, 10, "")
	require.NoError(t, err)

	// 图标回落：菜单不配 icon，挂载一个带 icon 的已发布页面
	require.NoError(t, seedConsolePublishedIconPage(service.svcCtx, ctx, "iconpage", "GiftOutlined", 1))
	beta, err := seedConsoleMenuByKey(service.svcCtx, ctx, "beta")
	require.NoError(t, err)
	require.NoError(t, seedConsolePageSpecMount(service.svcCtx, ctx, "iconpage", &beta.ID))

	// 挂载到菜单但未发布（draft-only）的页面不进树
	require.NoError(t, seedConsolePageSpecMount(service.svcCtx, ctx, "draftpage", &beta.ID))

	// 未挂载的已发布页面（menuID nil）不进树
	require.NoError(t, seedConsolePublishedIconPage(service.svcCtx, ctx, "freepage", "GiftOutlined", 2))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)

	// 同序按本地化标题：乙(U+4E59) 码点先于 甲(U+7532)，故 乙组(beta/gamma)
	// 排在 甲组(alpha) 之前；标题同 → key beta < gamma
	assert.Equal(t, "beta", resp.Items[0].Key)
	assert.Equal(t, "gamma", resp.Items[1].Key)
	assert.Equal(t, "alpha", resp.Items[2].Key)

	// beta 组：挂载且已发布的 iconpage 进入；draftpage/freepage 不进
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "iconpage", resp.Items[0].Children[0].Key)
	// 菜单未配 icon → 回落组内第一个非空页面图标
	assert.Equal(t, "GiftOutlined", resp.Items[0].Icon)

	// en-US 请求：gamma 命中 en-US；无 en-US 的 alpha 回落 zh-CN
	respEN, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "en-US"})
	require.NoError(t, err)
	require.Len(t, respEN.Items, 3)
	for _, item := range respEN.Items {
		switch item.Key {
		case "gamma":
			assert.Equal(t, "Beta", getLocalizedText(item.Title, "en-US", item.Key), "en-US 直接命中")
		case "alpha":
			assert.Equal(t, "甲组", getLocalizedText(item.Title, "en-US", item.Key), "无 en-US 回落 zh-CN")
		}
	}
}
