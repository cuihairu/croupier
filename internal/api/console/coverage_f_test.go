package console

import (
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/api/menu"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 本文件收敛 generateMenuFromMenuItems / consoleMenuItemFromMenu /
// getLocalizedText 的残余分支。错误注入沿用包内既定手法（gorm 回调按
// 表名注错；主回调在 db.Error != nil 时短路，SQL 不真正下发）。

// coverageFTable 取当前语句的目标表名（Statement.Table 为空时回退已解析
// schema 的表名）。
func coverageFTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

// generateMenuFromMenuItems 的菜单树读取错误分支：menu_items 查询注入
// 失败 → AccessibleTree 报错 → 整体失败（与整库 Close 不同，前置的
// 身份/角色查询不受影响，错误精确落在菜单树读取一行）。
func TestCoverageFGenerateMenuItemsQueryFailure(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)

	require.NoError(t, service.svcCtx.DB.Callback().Query().Before("gorm:Query").Register("coverage_f_fail_menu_items", func(tx *gorm.DB) {
		if coverageFTable(tx) == "menu_items" {
			tx.AddError(errors.New("coverageF injected query failure on menu_items"))
		}
	}))

	_, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "menu_items")
}

// 挂载映射循环的 menuID nil 分支：draft page_specs 行未挂任何菜单
// （menu_id 为 NULL）→ continue 跳过，菜单树不受影响。
func TestCoverageFSkipUnattachedDraftPage(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := seedConsoleMenu(service.svcCtx, ctx, "player", spec.LocalizedText{"zh-CN": "玩家"}, 1, "")
	require.NoError(t, err)
	// menuID 传 nil：draft 行存在但没有挂载目标。
	require.NoError(t, seedConsolePageSpecMount(service.svcCtx, ctx, "floating", nil))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Empty(t, resp.Items[0].Children, "未挂载的 draft 页面不进树")
}

// consoleMenuItemFromMenu 的子项排序决胜链：Order 相同时先比本地化标题，
// 标题再同比 key。五个子项全部 Order=5——菜单子项 ka/kv/kt 与挂载页
// page.t/page.z。码点序「乙」(U+4E59) < 「同」(U+540C) < 「甲」(U+7532)：
//   - page.z(乙) 与「同题」组比较命中 title 决胜（left != right）；
//   - ka/kv/page.t 三项标题全同，命中 key 决胜（left == right）。
func TestCoverageFConsoleMenuItemChildrenTieBreak(t *testing.T) {
	node := &menu.MenuDTO{
		ID:        7,
		MenuKey:   "group",
		Labels:    spec.LocalizedText{"zh-CN": "分组"},
		SortOrder: 1,
		IsVisible: true,
		Children: []*menu.MenuDTO{
			{ID: 1, MenuKey: "kv", Labels: spec.LocalizedText{"zh-CN": "同题"}, SortOrder: 5, IsVisible: true},
			{ID: 2, MenuKey: "ka", Labels: spec.LocalizedText{"zh-CN": "同题"}, SortOrder: 5, IsVisible: true},
			{ID: 3, MenuKey: "kt", Labels: spec.LocalizedText{"zh-CN": "甲"}, SortOrder: 5, IsVisible: true},
		},
	}
	pages := map[uint][]pageEntry{
		7: {
			{key: "page.t", title: spec.LocalizedText{"zh-CN": "同题"}, order: 5},
			{key: "page.z", title: spec.LocalizedText{"zh-CN": "乙"}, order: 5},
		},
	}

	item := consoleMenuItemFromMenu(node, pages, "zh-CN")
	keys := make([]string, 0, len(item.Children))
	for _, child := range item.Children {
		keys = append(keys, child.Key)
	}
	assert.Equal(t, []string{"page.z", "ka", "kv", "page.t", "kt"}, keys)
}

// getLocalizedText 的 en-US 回落分支：请求语言与 zh-CN 均未命中时取 en-US。
func TestCoverageFGetLocalizedTextEnglishFallback(t *testing.T) {
	assert.Equal(t, "Only", getLocalizedText(spec.LocalizedText{"en-US": "Only"}, "zh-CN", "fb"))
}
