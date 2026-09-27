package registry_test

// #11 元数据 EAV 的覆盖率巡检补测（外部测试包）：缓存命中/缺失表降级/
// 零值与 nil 接收者守卫。错误路径只告警不阻断（元数据是观测数据），这里
// 验证降级行为本身。

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	registry "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newMetadataPartialTestDB 只迁移 agent_sessions、故意不建 provider_metadata：
// 用于模拟存量库迁移未跑/表缺失时聚合与注册投影的降级路径。
func newMetadataPartialTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta-partial.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registry.AgentSessionDB{}))
	return db
}

// nil 接收者：方法带 s == nil 守卫，返回 nil 而非 panic。
func TestProviderMetaOptionsNilReceiverReturnsNil(t *testing.T) {
	var s *registry.Store
	assert.Nil(t, s.ProviderMetaOptions("game-a", "dev"))
}

// 零值 Store（不经 NewStore 构造）：metaOptions 惰性初始化 + DB-less 内存
// 聚合空集路径。
func TestProviderMetaOptionsZeroValueStoreYieldsEmpty(t *testing.T) {
	s := &registry.Store{}
	items := s.ProviderMetaOptions("game-a", "dev")
	assert.Empty(t, items)
	// 初始化后再查仍为空（缓存写入/读取往返）。
	assert.Empty(t, s.ProviderMetaOptions("game-a", "dev"))
}

// 30s TTL 内缓存命中：表数据被清空后立即再查仍返回首次结果（来自缓存，
// 不落库重算）。
func TestProviderMetaOptionsCacheHitWithinTTL(t *testing.T) {
	db := newMetadataTestDB(t)
	store := registry.NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []registry.ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
		},
	}))

	first := store.ProviderMetaOptions("game-a", "dev")
	require.Len(t, metaOptionValues(first, "serverId"), 1)

	// 绕过注册链路直清表：TTL 内的再查必须命中缓存而非重算。
	require.NoError(t, db.Where("1 = 1").Delete(&model.ProviderMetadata{}).Error)
	cached := store.ProviderMetaOptions("game-a", "dev")
	require.Len(t, metaOptionValues(cached, "serverId"), 1,
		"within TTL the cached aggregation must be served even if table is emptied")
}

// provider_metadata 表缺失（存量库迁移未跑）：注册投影与聚合均降级为
// 告警+空结果，不 panic、不阻断注册主链。
func TestProviderMetaOptionsMissingTableDegradesGracefully(t *testing.T) {
	db := newMetadataPartialTestDB(t)
	store := registry.NewStoreWithDB(db)

	// 注册成功（agent_sessions 正常），元数据投影失败仅告警。
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []registry.ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
		},
	}))

	items := store.ProviderMetaOptions("game-a", "dev")
	assert.Empty(t, items, "missing table must degrade to empty options, not panic")
}
