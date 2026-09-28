package registry

// 覆盖率巡检补测（store_metadata.go 97.3%，4 块）：provider 元数据增量
// 判定的「集合等长但内容不同」翼、stale service 收集的 prev 重复去重、
// 内存聚合的 env 过滤翼、选项分组的实例数降序比较翼。
// 同包测试（操作未导出字段 agents 与未导出类型 metaKeyValueCount）；
// 不 import internal/model（包边界既有铁律）。

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 集合等长时仍需逐 ID 对比：ID 不同（!ok 翼）或同 ID 键数不同（len 翼）
// 都必须判为「有变化」，否则重复注册会漏写 DB。
func TestProviderMetadataDelta_EqualSizeContentDrift(t *testing.T) {
	prev := []ProviderSession{
		{ProviderID: "svc-a", Metadata: map[string]string{"k1": "v1"}},
	}
	cur := []ProviderSession{
		{ProviderID: "svc-b", Metadata: map[string]string{"k1": "v1"}},
	}
	assert.True(t, providerMetadataDelta(prev, cur), "等长但 ID 集合不同应为增量")

	cur = []ProviderSession{
		{ProviderID: "svc-a", Metadata: map[string]string{"k1": "v1", "k2": "v2"}},
	}
	assert.True(t, providerMetadataDelta(prev, cur), "同 ID 键数变化应为增量")

	same := []ProviderSession{{ProviderID: "svc-a", Metadata: map[string]string{"k1": "v1"}}}
	assert.False(t, providerMetadataDelta(same, same), "完全一致应判无增量（跳过写放大）")
}

// prev 中重复出现的消失 service 只收集一次（seen 去重翼）。
func TestStaleServiceIDs_DedupsPrevDuplicates(t *testing.T) {
	prev := []ProviderSession{
		{ProviderID: "svc-a"},
		{ProviderID: "svc-a"}, // 重复注册形态：同一 service 出现两次
		{ProviderID: "svc-b"},
	}
	cur := []ProviderSession{{ProviderID: "svc-b"}}

	gone := staleServiceIDs(prev, cur)
	require.Len(t, gone, 1, "重复的消失 service 只删一次: %v", gone)
	assert.Equal(t, "svc-a", gone[0])
}

// DB-less 内存聚合的 env 过滤翼：跨 env 快照不得混入当前 scope 聚合。
func TestProviderMetaOptions_MemoryAggregateEnvFilter(t *testing.T) {
	store := &Store{agents: map[string]*AgentSession{
		"agent-dev": {AgentID: "agent-dev", GameID: "game-a", Env: "dev",
			Providers: []ProviderSession{{ProviderID: "svc-dev", Metadata: map[string]string{"region": "cn"}}}},
		"agent-prod": {AgentID: "agent-prod", GameID: "game-a", Env: "prod",
			Providers: []ProviderSession{{ProviderID: "svc-prod", Metadata: map[string]string{"region": "us"}}}},
	}}

	items := store.ProviderMetaOptions("game-a", "dev")
	require.Len(t, items, 1, "prod 快照被 env 过滤后仅剩 dev: %+v", items)
	require.Len(t, items[0].Values, 1)
	assert.Equal(t, "region", items[0].Key)
	assert.Equal(t, "cn", items[0].Values[0].Value)
	assert.Equal(t, 1, items[0].Values[0].Count)
}

// 选项分组：键内值按实例数降序（3>2>1），同数按值字典序。
func TestGroupMetaOptions_SortsValuesByCountDesc(t *testing.T) {
	rows := []metaKeyValueCount{
		{key: "region", value: "rare", count: 1},
		{key: "region", value: "major", count: 3},
		{key: "lang", value: "go", count: 5},
		{key: "region", value: "mid", count: 2},
	}
	items := groupMetaOptions(rows)
	require.Len(t, items, 2)
	assert.Equal(t, "lang", items[0].Key, "键按字典序")
	assert.Equal(t, "region", items[1].Key)

	values := items[1].Values
	require.Len(t, values, 3)
	assert.Equal(t, []string{"major", "mid", "rare"},
		[]string{values[0].Value, values[1].Value, values[2].Value}, "实例数降序")
	assert.Equal(t, []int{3, 2, 1}, []int{values[0].Count, values[1].Count, values[2].Count})
}
