package registry_test

// #11 回归：provider 元数据 EAV 持久化 + 去重键值聚合。
// 此前元数据纯内存态（重启即失）；本组用例覆盖：注册投影落库、重启后
// （新 Store 挂同一 DB）聚合可查、消失 service 行清理、值覆盖更新、
// scope 隔离、DB-less 在线会话聚合退化路径。

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMetadataTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&registry.AgentSessionDB{},
		&model.ProviderMetadata{},
	))
	return db
}

func metaOptionValues(items []registry.ProviderMetaKeyOption, key string) []registry.ProviderMetaValueOption {
	for _, item := range items {
		if item.Key == key {
			return item.Values
		}
	}
	return nil
}

func TestProviderMetadataRefreshPersistsAcrossRestart(t *testing.T) {
	db := newMetadataTestDB(t)
	store := registry.NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []registry.ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1", "region": "cn"}},
			{ProviderID: "svc-2", Metadata: map[string]string{"serverId": "s-1"}},
		},
	}))

	// 模拟 server 重启：同一 DB 挂新 Store，内存会话为空。
	restarted := registry.NewStoreWithDB(db)
	items := restarted.ProviderMetaOptions("game-a", "dev")
	require.NotEmpty(t, items, "options must survive restart via EAV table")

	serverID := metaOptionValues(items, "serverId")
	require.Len(t, serverID, 1)
	assert.Equal(t, "s-1", serverID[0].Value)
	// 两个实例报同一 serverId → distinct service_id 计数为 2。
	assert.Equal(t, 2, serverID[0].Count)

	region := metaOptionValues(items, "region")
	require.Len(t, region, 1)
	assert.Equal(t, "cn", region[0].Value)
	assert.Equal(t, 1, region[0].Count)
}

func TestProviderMetadataRefreshUpdatesValuesAndCleansGoneServices(t *testing.T) {
	db := newMetadataTestDB(t)
	store := registry.NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []registry.ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "old-1"}},
			{ProviderID: "svc-gone", Metadata: map[string]string{"serverId": "gone"}},
		},
	}))

	// 二次注册：svc-1 值变化、svc-gone 消失、svc-new 出现。
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []registry.ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "new-1"}},
			{ProviderID: "svc-new", Metadata: map[string]string{"serverId": "old-1"}},
		},
	}))

	items := store.ProviderMetaOptions("game-a", "dev")
	serverID := metaOptionValues(items, "serverId")
	require.Len(t, serverID, 2, "values should collapse to new-1/old-1 after refresh")
	assert.Equal(t, "new-1", serverID[0].Value)
	assert.Equal(t, "old-1", serverID[1].Value)

	// svc-gone 的行必须被清掉：无任何键下残留 gone 值。
	for _, item := range items {
		for _, v := range item.Values {
			assert.NotEqual(t, "gone", v.Value, "removed service row must be deleted")
		}
	}
}

func TestProviderMetadataOptionsScopeIsolation(t *testing.T) {
	db := newMetadataTestDB(t)
	store := registry.NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-a",
		GameID:    "game-a",
		Env:       "dev",
		Providers: []registry.ProviderSession{{ProviderID: "svc-a", Metadata: map[string]string{"serverId": "s-a"}}},
	}))
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-b",
		GameID:    "game-b",
		Env:       "prod",
		Providers: []registry.ProviderSession{{ProviderID: "svc-b", Metadata: map[string]string{"serverId": "s-b"}}},
	}))

	aItems := store.ProviderMetaOptions("game-a", "dev")
	require.Len(t, metaOptionValues(aItems, "serverId"), 1)
	assert.Equal(t, "s-a", metaOptionValues(aItems, "serverId")[0].Value)

	bItems := store.ProviderMetaOptions("game-b", "prod")
	assert.Equal(t, "s-b", metaOptionValues(bItems, "serverId")[0].Value)

	// 无 scope → 全量聚合（跨 scope 汇总）。
	all := store.ProviderMetaOptions("", "")
	assert.Len(t, metaOptionValues(all, "serverId"), 2)
}

func TestProviderMetadataDBLessFallsBackToLiveSessions(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "agent-a",
		GameID:    "game-a",
		Env:       "dev",
		Providers: []registry.ProviderSession{{ProviderID: "svc-a", Metadata: map[string]string{"serverId": "s-a"}}},
	}))

	items := store.ProviderMetaOptions("game-a", "dev")
	serverID := metaOptionValues(items, "serverId")
	require.Len(t, serverID, 1)
	assert.Equal(t, "s-a", serverID[0].Value)

	// 另一 scope 不可泄漏。
	assert.Empty(t, metaOptionValues(store.ProviderMetaOptions("game-b", "dev"), "serverId"))
}

func TestProviderMetadataNoChangeRegistrationSkipsRewrite(t *testing.T) {
	db := newMetadataTestDB(t)
	store := registry.NewStoreWithDB(db)

	sess := &registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game-a",
		Env:       "dev",
		Providers: []registry.ProviderSession{{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}}},
	}
	require.NoError(t, store.UpsertAgent(sess))
	require.NoError(t, store.UpsertAgent(sess))

	var count int64
	require.NoError(t, db.Table("provider_metadata").Count(&count).Error)
	assert.Equal(t, int64(1), count, "no-change re-registration must not duplicate rows")
}
