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

// #27②③：运行时导入条目补充实例元数据、被调用方地址、导入时间与最新
// 版本高水位；firstSeenUnix/latestVersion 由服务端归一（无观测值回退
// lastSeenUnix/version），前端无需判零值/空串。
func TestRuntimeSourcesExposesMetadataAddrAndVersionHistory(t *testing.T) {
	service, ctx := setupOpenAPITestServiceWithPermissions(t, "openapi_sources:read")
	store := service.svcCtx.RegistryStore
	now := time.Now()
	firstSeen := now.Add(-2 * time.Hour).Unix()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "openapi-demo-agent",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
		Providers: []registry.ProviderSession{{
			ProviderID:    "provider:players",
			Addr:          "10.0.0.8:9001",
			Version:       "1.1.0",
			VersionHWM:    "1.4.0",
			FirstSeenUnix: firstSeen,
			Metadata:      map[string]string{"serverId": "s1"},
			LastSeenUnix:  now.Unix(),
			FunctionIDs:   []string{"players.player.list"},
		}},
	}))
	// 快照零值归一：FirstSeenUnix=0 / VersionHWM="" 的会话回退 lastSeen/version
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:  "fresh-agent",
		GameID:   "demo-game",
		Env:      "development",
		LastSeen: now,
		Providers: []registry.ProviderSession{{
			ProviderID:   "provider:battles",
			Addr:         "10.0.0.9:9002",
			Version:      "0.3.0",
			LastSeenUnix: now.Unix(),
			FunctionIDs:  []string{"battles.battle.start"},
		}},
	}))

	resp, err := service.RuntimeSources(ctx, &RuntimeSourcesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)

	byID := map[string]RuntimeProviderItem{}
	for _, item := range resp.Items {
		byID[item.ProviderID] = item
	}

	players := byID["provider:players"]
	assert.Equal(t, "10.0.0.8:9001", players.ServiceAddr)
	assert.Equal(t, map[string]string{"serverId": "s1"}, players.Metadata)
	assert.Equal(t, firstSeen, players.FirstSeenUnix)
	assert.Equal(t, "1.4.0", players.LatestVersion, "有高水位时取高水位")
	assert.Equal(t, "1.1.0", players.Version)

	battles := byID["provider:battles"]
	assert.Equal(t, "10.0.0.9:9002", battles.ServiceAddr)
	assert.Empty(t, battles.Metadata)
	assert.Equal(t, now.Unix(), battles.FirstSeenUnix, "零值归一回退 lastSeenUnix")
	assert.Equal(t, "0.3.0", battles.LatestVersion, "空高水位归一回退 version")
}
