package menu

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件固化 filterAccessibleTree 的 `!check(parent)` 防御分支（唯一豁免
// 残留，完整论证见 docs/development/coverage-exemptions.md）。该分支在当前
// 实现下恒不可达：check(item)=true 且父节点存在时，check 内部递归已执行
// ok=check(parent) 并为 true，主循环再次 check(parent) 命中缓存返回同一
// final 值。不删产品代码，以 pin 测试锁定其依赖的不变式（缓存一致性、
// 成环 false 传播、ParentID/byID/权限集执行期不可变）——若未来实现变化
// 导致分支变为可达，本文件的测试会失败并提醒补错误路径用例。

// SetLabels 恒返回 nil：map[string]string 的 json.Marshal 恒成功，error
// 返回值仅为对齐既有模型助手签名而保留（internal/model/menu.go 函数注释
// 自证）。Create/Update 里 `if err := item.SetLabels(...); err != nil` 的
// 错误出口因此不可达。本测试锁定「任意输入（nil / 空 / 合法）恒 nil」；
// 若 SetLabels 将来引入校验（如空 labels 报错），此处失败，届时必须为
// Create/Update 的错误出口补端到端用例。
func TestCoverageFSetLabelsAlwaysSucceeds(t *testing.T) {
	item := &model.MenuItem{}
	require.NoError(t, item.SetLabels(nil))
	require.NoError(t, item.SetLabels(map[string]string{}))
	require.NoError(t, item.SetLabels(map[string]string{"zh-CN": "玩家", "en-US": "Player"}))
	assert.Equal(t, "玩家", item.GetLabels()["zh-CN"])
}

// filterAccessibleTree 380-381（`if !check(parent) { continue }`）的可达性
// 论证：主循环到达 380 时必有 check(item)=true 且父节点存在于 byID。而
// check 的定义（348-352）里，item 可见有权限且父存在时 ok=check(parent)——
// 即 check(item)=true 已蕴含 check(parent)=true（缓存命中），380 的
// !check(parent) 恒为 false，continue 恒不执行。
// 本测试锁定该不变式赖以成立的脏数据语义：互环/自环节点整体不可见
// （check 先落 false 再递归，环上所有节点均被 368 行 continue）、孤儿
// （父记录不存在）提升为根、不可见父级隐藏整支子树——输出一致且无 panic。
// 若 380-381 将来变为可达（例如 check 语义改为不递归父级），本测试的断言
// 组合会暴露行为分歧。
func TestCoverageFFilterAccessibleTreeDefensiveInvariants(t *testing.T) {
	parent1, parent2 := uint(1), uint(2)
	selfRing := uint(3)
	orphan := uint(99)
	hidden := uint(7)
	items := []model.MenuItem{
		{ID: 1, MenuKey: "ring-a", ParentID: &parent2, IsVisible: true},
		{ID: 2, MenuKey: "ring-b", ParentID: &parent1, IsVisible: true},
		{ID: 3, MenuKey: "self-ring", ParentID: &selfRing, IsVisible: true},
		{ID: 4, MenuKey: "orphan", ParentID: &orphan, IsVisible: true},
		{ID: 5, MenuKey: "root", IsVisible: true},
		{ID: 6, MenuKey: "under-hidden", ParentID: &hidden, IsVisible: true},
		{ID: 7, MenuKey: "hidden", IsVisible: false},
	}

	roots := filterAccessibleTree(items, nil)

	keys := make([]string, 0, len(roots))
	for _, root := range roots {
		keys = append(keys, root.MenuKey)
	}
	// 互环 A↔B、自环、隐藏父级及其子级均不出现；孤儿提升为根。
	// 顺序按 items 遍历序：orphan(4) 先于 root(5)。
	assert.Equal(t, []string{"orphan", "root"}, keys)
	for _, root := range roots {
		assert.Empty(t, root.Children, "%s 不应有任何子节点", root.MenuKey)
	}
}

// Update 传全空白 labels 时 validateLabels 报「菜单名称不能为空」——
// 覆盖 Update 内 validateLabels 的失败分支（该分支在 SetLabels 恒 nil
// 论证之外，是可达的真实校验路径）。
func TestCoverageFUpdateBlankLabelsRejected(t *testing.T) {
	service, ctx := newMenuTestService(t, "menu:create", "menu:update")

	created, err := service.Create(ctx, &CreateMenuRequest{MenuKey: "blank-labels", Labels: menuLabels()})
	require.NoError(t, err)

	blank := spec.LocalizedText{"zh-CN": "   "}
	_, err = service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), Labels: &blank})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "菜单名称不能为空")

	// 合法 labels 走通 validateLabels → SetLabels 落库全链路。
	rename := spec.LocalizedText{"zh-CN": "改名后的菜单", "en-US": "Renamed"}
	updated, err := service.Update(ctx, &UpdateMenuRequest{ID: formatID(created.ID), Labels: &rename})
	require.NoError(t, err)
	assert.Equal(t, map[string]string(rename), map[string]string(updated.Labels))
}
