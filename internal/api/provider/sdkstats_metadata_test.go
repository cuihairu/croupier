// SdkStats 实例元数据回归：metadata 透传到实例明细 + metaKey/metaValue
// 服务端子串过滤（value 条件同时匹配键名；key+value 须同一 KV 对命中）。
package provider

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMetadataStatsStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-meta",
		GameID:  "game-meta",
		Env:     "dev",
		Providers: []reg.ProviderSession{
			{
				ProviderID:  "game-demo",
				SDKLanguage: "go",
				SDKVersion:  "1.4.0",
				Metadata:    map[string]string{"serverId": "s1", "pod": "game-7c4d"},
			},
			{
				ProviderID:  "prom-adapter",
				SDKLanguage: "js",
				SDKVersion:  "1.5.0",
				Metadata:    map[string]string{"serverId": "s9"},
			},
			{
				ProviderID:  "bare-adapter",
				SDKLanguage: "python",
				SDKVersion:  "0.1.0",
			},
		},
	}); err != nil {
		panic(err)
	}
	return store
}

func findInstanceItem(t *testing.T, items []SdkInstanceItem, providerID string) SdkInstanceItem {
	t.Helper()
	for _, item := range items {
		if item.ProviderID == providerID {
			return item
		}
	}
	t.Fatalf("instance %q not found", providerID)
	return SdkInstanceItem{}
}

func TestServiceSdkStats_MetadataPassthrough(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Instances, 3)

	demo := findInstanceItem(t, resp.Instances, "game-demo")
	assert.Equal(t, map[string]string{"serverId": "s1", "pod": "game-7c4d"}, demo.Metadata)
	// 无元数据实例字段为 nil（wire omitempty 不发空 map）
	assert.Nil(t, findInstanceItem(t, resp.Instances, "bare-adapter").Metadata)
	assert.Equal(t, 3, resp.TotalInstances)
}

func TestServiceSdkStats_MetaKeyFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// metaKey=serverid（大小写不敏感）→ 两个带 serverId 的实例
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "ServerID "})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"game-demo", "prom-adapter"}, providerIDs(resp.Instances))

	// metaKey=pod → 仅 game-demo
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "pod"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// metaKey 无命中 → 空结果
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "region"})
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
	assert.Equal(t, 0, resp.TotalInstances)
}

func TestServiceSdkStats_MetaValueFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// 直接粘值 s1 → game-demo
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "s1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// value 条件同时匹配键名：粘 "pod" 也能命中带 pod 键的实例
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "POD"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))

	// k=v 整对作为 value 亦可命中（前端 haystack 同构）
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaValue: "serverId=s9"})
	require.NoError(t, err)
	assert.Equal(t, []string{"prom-adapter"}, providerIDs(resp.Instances))
}

func TestServiceSdkStats_MetaKeyAndValueMustHitSamePair(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newMetadataStatsStore()})

	// key 与 value 都存在但分散在两个 KV 对上 → 不命中
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "serverId", MetaValue: "game-7c4d"})
	require.NoError(t, err)
	assert.Empty(t, providerIDs(resp.Instances))

	// 同一 KV 对同时命中 → game-demo
	resp, err = s.SdkStats(context.Background(), &SdkStatsRequest{MetaKey: "serverId", MetaValue: "s1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"game-demo"}, providerIDs(resp.Instances))
}

func TestMatchMetadata_SubstringSemantics(t *testing.T) {
	// 空条件恒真；无元数据遇条件恒假
	assert.True(t, matchMetadata(nil, "", ""))
	assert.False(t, matchMetadata(nil, "k", ""))
	assert.False(t, matchMetadata(map[string]string{}, "", "v"))

	md := map[string]string{"serverId": "s1"}
	assert.True(t, matchMetadata(md, "server", "1"))
	assert.True(t, matchMetadata(md, "SERVERID", "S1"))
	assert.False(t, matchMetadata(md, "serverId", "s2"))
}

func providerIDs(items []SdkInstanceItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ProviderID)
	}
	return out
}
