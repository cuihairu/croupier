package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// collectDescendants 是删除级联的核心：漏收集 = 孤儿菜单行；误收集 = 误删。

func TestCollectDescendants_TreeAndLeaf(t *testing.T) {
	// 1 → 2,3；2 → 4；5 是旁支（不应被 1 的删除波及）。
	children := map[uint][]uint{
		1: {2, 3},
		2: {4},
		5: {6},
	}

	got := collectDescendants(1, children)
	assert.ElementsMatch(t, []uint{2, 3, 4}, got)

	// 叶节点 → 空。
	assert.Empty(t, collectDescendants(4, children))
	assert.Empty(t, collectDescendants(6, children))

	// 不存在的根 → 空。
	assert.Empty(t, collectDescendants(99, children))

	// 空 map。
	assert.Empty(t, collectDescendants(1, nil))
}

func TestCollectDescendants_DeepChainNoCycleSafe(t *testing.T) {
	// 线性链 1→2→3→4→5。
	children := map[uint][]uint{
		1: {2},
		2: {3},
		3: {4},
		4: {5},
	}
	assert.ElementsMatch(t, []uint{2, 3, 4, 5}, collectDescendants(1, children))

	// 已知边界：实现是无 visited 集的 DFS，脏数据成环会无限展开——
	// 生产路径由 parent 指针树保证无环，此处不测环（避免挂起）。
}
