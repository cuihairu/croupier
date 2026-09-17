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

// newMenuAwarePageService 在 page 测试服务上补齐 MenuModel（菜单关联用），
// 并返回带 scope/身份的 context（与 newPageTestService 同源常量）。
func newMenuAwarePageService(t *testing.T, permissions ...string) (*Service, context.Context, *model.MenuItemModel) {
	t.Helper()
	service, _, _ := newPageTestService(t, permissions...)
	menuModel := model.NewMenuItemModel(service.svcCtx.DB)
	service.svcCtx.MenuModel = menuModel
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	ctx = context.WithValue(ctx, "username", "page_tester")
	return service, ctx, menuModel
}

func createTestMenu(t *testing.T, menuModel *model.MenuItemModel, ctx context.Context, menuKey string) *model.MenuItem {
	t.Helper()
	menu := &model.MenuItem{
		GameID:    "demo-game",
		Env:       "development",
		MenuKey:   menuKey,
		IsVisible: true,
	}
	require.NoError(t, menuModel.Create(ctx, menu))
	return menu
}

func ptrInt64(v int64) *int64 { return &v }

func TestSetPageMenuMountAndReadBack(t *testing.T) {
	service, ctx, menuModel := newMenuAwarePageService(t, "pages:edit", "pages:read")
	menu := createTestMenu(t, menuModel, ctx, "resource")
	saveTestPageDraft(t, service, ctx)

	resp, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.NoError(t, err)
	require.NotNil(t, resp.MenuID)
	assert.EqualValues(t, menu.ID, *resp.MenuID)

	// GetDraft 回读
	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.NotNil(t, draft.MenuID)
	assert.EqualValues(t, menu.ID, *draft.MenuID)

	// ListDrafts 摘要回读
	list, err := service.ListDrafts(ctx, &PageDraftListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.NotNil(t, list.Items[0].MenuID)
	assert.EqualValues(t, menu.ID, *list.Items[0].MenuID)
}

func TestSetPageMenuUnmount(t *testing.T) {
	service, ctx, menuModel := newMenuAwarePageService(t, "pages:edit", "pages:read")
	menu := createTestMenu(t, menuModel, ctx, "resource")
	saveTestPageDraft(t, service, ctx)

	// null 解除
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: nil})
	require.NoError(t, err)

	// 挂载后 0 解除
	_, err = service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.NoError(t, err)
	zero := int64(0)
	resp, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: &zero})
	require.NoError(t, err)
	assert.Nil(t, resp.MenuID)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Nil(t, draft.MenuID)
}

func TestSetPageMenuValidation(t *testing.T) {
	service, ctx, _ := newMenuAwarePageService(t, "pages:edit", "pages:read")
	saveTestPageDraft(t, service, ctx)

	// 菜单不存在 → 404
	missing := int64(99999)
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: &missing})
	require.Error(t, err)

	// 页面不存在
	menuID := int64(1)
	_, err = service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "no.such.page", MenuID: &menuID})
	require.Error(t, err)

	// 其他 scope 的菜单不可见
	menu := &model.MenuItem{GameID: "demo-game", Env: "production", MenuKey: "prodmenu"}
	require.NoError(t, service.svcCtx.MenuModel.Create(ctx, menu))
	_, err = service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.Error(t, err, "跨 env 菜单不可挂载")
}

func TestSaveDraftPreservesMenuAssociation(t *testing.T) {
	service, ctx, menuModel := newMenuAwarePageService(t, "pages:edit", "pages:read")
	menu := createTestMenu(t, menuModel, ctx, "resource")

	revision := saveTestPageDraft(t, service, ctx)
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: ptrInt64(int64(menu.ID))})
	require.NoError(t, err)

	// 再保存一次草稿：菜单关联必须保留
	nextRevision := revision
	_, err = service.SaveDraft(ctx, &PageSaveRequest{
		PageKey:       "player.manage",
		DraftRevision: &nextRevision,
		Type:          spec.PageTypeOperation,
		ResourceKey:   "player",
		Title:         map[string]string{"zh-CN": "玩家管理"},
		Category: spec.PageCategorySpec{
			Key:    "player",
			Labels: spec.LocalizedText{"zh-CN": "玩家", "en-US": "Player"},
		},
		Operation: testOperationPageSpec(),
		Bindings:  testPageBindings(),
	})
	require.NoError(t, err)

	draft, err := service.GetDraft(ctx, &PageDraftRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	require.NotNil(t, draft.MenuID, "SaveDraft 不得清除菜单关联")
	assert.EqualValues(t, menu.ID, *draft.MenuID)
}

func TestSetPageMenuPermissionDenied(t *testing.T) {
	service, ctx, _ := newMenuAwarePageService(t, "pages:read")

	menuID := int64(1)
	_, err := service.SetPageMenu(ctx, &PageMenuUpdateRequest{PageKey: "player.manage", MenuID: &menuID})
	require.Error(t, err, "无 pages:edit 权限应被拒绝")
}
