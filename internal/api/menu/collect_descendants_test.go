package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// collectDescendants is the core of the delete cascade: collecting too little
// leaves orphan menu rows, collecting too much deletes the wrong rows.

func TestCollectDescendants_TreeAndLeaf(t *testing.T) {
	// 1 → 2,3; 2 → 4; 5 is a side branch (must not be touched by deleting 1).
	children := map[uint][]uint{
		1: {2, 3},
		2: {4},
		5: {6},
	}

	got := collectDescendants(1, children)
	assert.ElementsMatch(t, []uint{2, 3, 4}, got)

	// leaf node → empty.
	assert.Empty(t, collectDescendants(4, children))
	assert.Empty(t, collectDescendants(6, children))

	// nonexistent root → empty.
	assert.Empty(t, collectDescendants(99, children))

	// empty map.
	assert.Empty(t, collectDescendants(1, nil))
}

func TestCollectDescendants_DeepChainNoCycleSafe(t *testing.T) {
	// Linear chain 1→2→3→4→5.
	children := map[uint][]uint{
		1: {2},
		2: {3},
		3: {4},
		4: {5},
	}
	assert.ElementsMatch(t, []uint{2, 3, 4, 5}, collectDescendants(1, children))

	// Known boundary: the implementation is a DFS without a visited set, so
	// dirty data forming a cycle would expand forever — the production path
	// guarantees acyclicity via the parent pointer tree, so cycles are not
	// tested here (avoiding a hang).
}
