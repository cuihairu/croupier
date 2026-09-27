// #38 回归：SdkStats 按中间件注入的游戏 scope 过滤实例。
// SDK 与游戏绑定，页面只看顶栏选中的 (game, env)；scope 缺失时保持
// 全量（service 直调/内部消费者的既有行为）。
package provider

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newScopedStatsStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-a",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []reg.ProviderSession{
			{ProviderID: "a-go", SDKLanguage: "go", SDKVersion: "1.4.0"},
			{ProviderID: "a-js", SDKLanguage: "js", SDKVersion: "1.5.0"},
		},
	}); err != nil {
		panic(err)
	}
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID: "agent-b",
		GameID:  "game-b",
		Env:     "prod",
		Providers: []reg.ProviderSession{
			{ProviderID: "b-py", SDKLanguage: "python", SDKVersion: "0.9.0"},
		},
	}); err != nil {
		panic(err)
	}
	return store
}

func sdkLanguageNames(resp *SdkStatsResponse) []string {
	out := make([]string, 0, len(resp.Languages))
	for _, item := range resp.Languages {
		out = append(out, item.Language)
	}
	return out
}

func TestServiceSdkStats_ScopeFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedStatsStore()})

	// 无 scope → 全量（历史行为，其他 service 直调方不受影响）
	resp, err := s.SdkStats(context.Background(), &SdkStatsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 3, resp.TotalInstances)

	// scope=game-a/dev → 仅该游戏实例；总数与语言聚合同步收窄
	resp, err = s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "dev"}),
		&SdkStatsRequest{},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"a-go", "a-js"}, providerIDs(resp.Instances))
	assert.Equal(t, 2, resp.TotalInstances)
	assert.ElementsMatch(t, []string{"go", "js"}, sdkLanguageNames(resp))

	// 游戏相同但 env 不同 → 一并过滤（game-a 没有 prod 会话）
	resp, err = s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"}),
		&SdkStatsRequest{},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
	assert.Equal(t, 0, resp.TotalInstances)
}

func TestServiceSdkStats_ScopeFilterWithMetaCondition(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedStatsStore()})

	// scope 过滤与 metaKey/metaValue 过滤叠加：game-b 里没有 serverId=s1
	resp, err := s.SdkStats(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"}),
		&SdkStatsRequest{MetaKey: "serverId", MetaValue: "s1"},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Instances)
}

// #2/#11 回归：MetaOptions 按中间件注入的游戏 scope 聚合实例元数据
// 去重键值（下拉选项服务端提供）；scope 缺失时保持全量。
func TestServiceMetaOptions_ScopeFilter(t *testing.T) {
	s := NewService(&svc.ServiceContext{RegistryStore: newScopedMetaStore()})

	resp, err := s.MetaOptions(
		svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "dev"}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "serverId", resp.Items[0].Key)
	require.Len(t, resp.Items[0].Values, 1)
	assert.Equal(t, "s-a", resp.Items[0].Values[0].Value)
	assert.Equal(t, 1, resp.Items[0].Values[0].Count)

	// 无 scope → 全量聚合（两游戏各一值）。
	resp, err = s.MetaOptions(context.Background())
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Len(t, resp.Items[0].Values, 2)
}

func newScopedMetaStore() *reg.Store {
	store := reg.NewStore()
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-a",
		GameID:    "game-a",
		Env:       "dev",
		Providers: []reg.ProviderSession{{ProviderID: "a-go", Metadata: map[string]string{"serverId": "s-a"}}},
	}); err != nil {
		panic(err)
	}
	if err := store.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-b",
		GameID:    "game-b",
		Env:       "prod",
		Providers: []reg.ProviderSession{{ProviderID: "b-py", Metadata: map[string]string{"serverId": "s-b"}}},
	}); err != nil {
		panic(err)
	}
	return store
}
