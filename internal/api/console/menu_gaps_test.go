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

// An error branch of generateMenuFromMenuItems: the menu tree is readable but the
// page table is missing -> the whole call errors.
func TestServiceMenuPageSpecTableMissing(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("page_specs"))
	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// The published snapshot table is missing -> the whole call errors (both the menu
// tree and the draft table remain readable).
func TestServiceMenuPublishedTableMissing(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Migrator().DropTable("published_page_specs"))
	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// DB fault injection: reading the menu tree fails -> Menu errors (no panic).
func TestServiceMenuDBClosed(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	sqlDB, err := service.svcCtx.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
}

// seedConsolePublishedIconPage publishes a page with an icon (dedicated to the icon
// fallback branch).
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

// Assembly details: equally-ordered menus sort by localized title/key, a menu
// without an icon falls back to a page icon inside the group, en-US copy falls
// back, and mounted-but-unpublished pages stay out of the tree (the menuID nil /
// no-match branches).
func TestServiceMenuAssemblyOrderingAndIconFallback(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")

	// Two menus with the same order: the title decides the order
	_, err := seedConsoleMenu(service.svcCtx, ctx, "beta", spec.LocalizedText{"zh-CN": "乙组"}, 10, "")
	require.NoError(t, err)
	_, err = seedConsoleMenu(service.svcCtx, ctx, "alpha", spec.LocalizedText{"zh-CN": "甲组"}, 10, "")
	require.NoError(t, err)
	// A third menu with the same order and the same title: the key decides the order
	// (beta < gamma)
	_, err = seedConsoleMenu(service.svcCtx, ctx, "gamma", spec.LocalizedText{"zh-CN": "乙组", "en-US": "Beta"}, 10, "")
	require.NoError(t, err)

	// Icon fallback: the menu has no icon, and a published page with an icon is
	// mounted under it
	require.NoError(t, seedConsolePublishedIconPage(service.svcCtx, ctx, "iconpage", "GiftOutlined", 1))
	beta, err := seedConsoleMenuByKey(service.svcCtx, ctx, "beta")
	require.NoError(t, err)
	require.NoError(t, seedConsolePageSpecMount(service.svcCtx, ctx, "iconpage", &beta.ID))

	// A page mounted to a menu but not published (draft-only) stays out of the tree
	require.NoError(t, seedConsolePageSpecMount(service.svcCtx, ctx, "draftpage", &beta.ID))

	// A published page that is not mounted (menuID nil) stays out of the tree
	require.NoError(t, seedConsolePublishedIconPage(service.svcCtx, ctx, "freepage", "GiftOutlined", 2))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)

	// Equally-ordered menus sort by localized title: the 乙(U+4E59) codepoint comes
	// before 甲(U+7532), so 乙组 (beta/gamma) sorts before 甲组 (alpha); equal titles
	// -> key beta < gamma
	assert.Equal(t, "beta", resp.Items[0].Key)
	assert.Equal(t, "gamma", resp.Items[1].Key)
	assert.Equal(t, "alpha", resp.Items[2].Key)

	// beta group: the mounted and published iconpage gets in; draftpage/freepage do not
	require.Len(t, resp.Items[0].Children, 1)
	assert.Equal(t, "iconpage", resp.Items[0].Children[0].Key)
	// The menu has no icon -> fall back to the first non-empty page icon in the group
	assert.Equal(t, "GiftOutlined", resp.Items[0].Icon)

	// en-US request: gamma hits en-US; alpha, which has no en-US, falls back to zh-CN
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
