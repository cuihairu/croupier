package openapi

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RuntimeSources 列出当前 scope 的运行时导入：仅 provider: 前缀会话、
// 按 scope 过滤、函数清单稳定排序并标注来源 Agent。
func TestRuntimeSourcesListsProviderSessionsInScope(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "openapi-demo-agent",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"players.player.list": {Enabled: true},
			"players.player.get":  {Enabled: true},
		},
		Providers: []registry.ProviderSession{{
			ProviderID:   "provider:players",
			Version:      "openapi-demo-1.0",
			LastSeenUnix: now.Unix(),
			FunctionIDs:  []string{"players.player.get", "players.player.list"},
		}},
		LastSeen: now,
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "plain-agent",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
	}))

	resp, err := service.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	item := resp.Items[0]
	assert.Equal(t, "provider:players", item.ProviderID)
	assert.Equal(t, "players", item.Name)
	assert.Equal(t, "openapi-demo-agent", item.AgentID)
	assert.Equal(t, "demo-game", item.GameID)
	assert.Equal(t, "development", item.Env)
	assert.Equal(t, 2, item.FunctionCount)
	assert.Equal(t, []string{"players.player.get", "players.player.list"}, item.Functions)
	assert.Equal(t, 1, resp.Total)
}

// 其他 scope 的 provider 会话不可见（scope 隔离）。
func TestRuntimeSourcesFiltersByScope(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "other-scope-agent",
		GameID:   "other-game",
		Env:      "prod",
		LastSeen: now,
		Providers: []registry.ProviderSession{{
			ProviderID:  "provider:players",
			FunctionIDs: []string{"players.player.list"},
		}},
	}))

	resp, err := service.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}
