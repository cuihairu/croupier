package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// 索引增删是列表过滤/标签聚合的底座：漏删 = 幽灵条目，误删 = 漏结果。

func newIndexedMeta() *functionv1.FunctionMetadata {
	return &functionv1.FunctionMetadata{
		Id:       "player.ban",
		Version:  "1.0.0",
		Resource: "player",
		Tags:     []string{"vip", "moderation"},
		Behavior: &functionv1.FunctionBehavior{
			Mode: functionv1.FunctionBehavior_MODE_QUERY,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel: functionv1.FunctionSecurity_RISK_LEVEL_HIGH,
		},
	}
}

func TestAddToIndexes_AndRemoveFromIndexes(t *testing.T) {
	s := NewStore()
	meta := newIndexedMeta()
	id := meta.Id

	s.addToIndexes(id, meta)
	assert.Contains(t, s.byResource["player"], id)
	assert.Contains(t, s.byTag["vip"], id)
	assert.Contains(t, s.byTag["moderation"], id)
	// RISK_LEVEL_HIGH → normalize → high
	assert.Contains(t, s.byRisk["high"], id)
	// MODE_QUERY → normalize → query
	assert.Contains(t, s.byMode["query"], id)

	// 二次 add 幂等（set 语义）。
	s.addToIndexes(id, meta)
	assert.Contains(t, s.byResource["player"], id)

	s.removeFromIndexes(id, meta)
	assert.NotContains(t, s.byResource["player"], id)
	// 空 set 应被整 key 清理。
	assert.NotContains(t, s.byResource, "player")
	assert.NotContains(t, s.byTag, "vip")
	assert.NotContains(t, s.byTag, "moderation")
	assert.NotContains(t, s.byRisk, "high")
	assert.NotContains(t, s.byMode, "query")
}

func TestAddToIndexes_EmptyResourceSkipped(t *testing.T) {
	s := NewStore()
	meta := &functionv1.FunctionMetadata{Id: "x"}
	s.addToIndexes("x", meta)
	assert.Empty(t, s.byResource)
	assert.Empty(t, s.byTag)
	assert.Empty(t, s.byRisk)
	assert.Empty(t, s.byMode)
	// 无 Security/Behavior 也不 panic。
	s.removeFromIndexes("x", meta)
}

func TestIntersectStringSets(t *testing.T) {
	a := map[string]struct{}{"x": {}, "y": {}, "z": {}}
	b := map[string]struct{}{"y": {}, "z": {}, "w": {}}
	got := intersectStringSets(a, b)
	assert.Len(t, got, 2)
	assert.Contains(t, got, "y")
	assert.Contains(t, got, "z")
	assert.NotContains(t, got, "x")
	assert.NotContains(t, got, "w")

	assert.Empty(t, intersectStringSets(nil, b))
	assert.Empty(t, intersectStringSets(a, nil))
	assert.Empty(t, intersectStringSets(
		map[string]struct{}{"p": {}},
		map[string]struct{}{"q": {}},
	))
}
