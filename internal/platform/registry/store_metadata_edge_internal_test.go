package registry

// #11 元数据 EAV 非导出守卫/错误路径补测（覆盖率巡检）。同包测试但不
// import internal/model（registry 包 import 边界：model 反向依赖本包，
// 同包 _test.go import model 同样环）。清理路径同款先例见
// store_metadata_cleanup_test.go。

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 只建 agent_sessions、不建 provider_metadata 的降级库。
func newEdgePartialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta-edge.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AgentSessionDB{}))
	return db
}

// 完整库：agent_sessions + provider_metadata（同包匿名行类型建表，
// 不经 internal/model——import 边界）。
func newMetadataTestDBInternal(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta-internal.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AgentSessionDB{}, &providerMetadataRow{}))
	return db
}

// invalidateMetaOptions 在零值 Store（map 未初始化）上必须安全返回。
func TestInvalidateMetaOptionsNilMapGuard(t *testing.T) {
	s := &Store{}
	s.invalidateMetaOptions("game-a", "dev") // 不 panic 即通过
}

// deleteProviderMetadataForServices 的三级守卫：nil 接收者 / nil 会话 /
// 无 providers——均应直接返回。
func TestDeleteProviderMetadataForServicesGuards(t *testing.T) {
	var nilStore *Store
	nilStore.deleteProviderMetadataForServices(nil) // nil 接收者

	s := &Store{}
	s.deleteProviderMetadataForServices(nil) // nil 会话

	s.deleteProviderMetadataForServices(&AgentSession{ // 有会话无 providers
		AgentID: "agent-1", GameID: "game-a", Env: "dev",
	})
}

// 会话过期清理在 provider_metadata 表缺失时：删除失败只告警，随后仍失效
// 选项缓存，不 panic、不影响清理主流程。
func TestDeleteProviderMetadataForServicesMissingTableWarnsNotPanics(t *testing.T) {
	store := NewStoreWithDB(newEdgePartialDB(t))
	store.deleteProviderMetadataForServices(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
		},
	})
}

// 跨 scope 重注册（agent 换 game）：same-scope 的旧行清理被跳过（false
// 分支），新 scope 正常投影——跨 scope 移动不在旧 scope 清行属现行为
// （会话过期清理按当前 scope 删）。
func TestRefreshProviderMetadataCrossScopeReRegisters(t *testing.T) {
	db := newMetadataTestDBInternal(t)
	store := NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
		},
	}))

	// 同 agent 改注册到 game-b：值也变化保证 delta 为真（触发刷新）。
	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-b",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-2"}},
		},
	}))

	newScope := store.ProviderMetaOptions("game-b", "dev")
	found := false
	for _, key := range newScope {
		for _, v := range key.Values {
			if key.Key == "serverId" && v.Value == "s-2" {
				found = true
			}
		}
	}
	require.True(t, found, "new-scope projection missing, got %v", newScope)
}

// 空/空白元数据键不投影（rows 为空时跳过 Create）。
func TestRefreshProviderMetadataSkipsBlankKeys(t *testing.T) {
	db := newMetadataTestDBInternal(t)
	store := NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"": "blank", "  ": "blank2"}},
		},
	}))

	items := store.ProviderMetaOptions("game-a", "dev")
	for _, key := range items {
		if key.Key == "" {
			t.Fatalf("blank key must not be projected, got %v", items)
		}
	}
}

// 同 scope 重注册且表中途被删（模拟存量库被外部动过）：消失 service 的
// 旧行删除与本次投影 upsert 双双失败均只告警，注册主链不受影响。
func TestRefreshProviderMetadataTableDroppedMidwayWarnsNotPanics(t *testing.T) {
	db := newMetadataTestDBInternal(t)
	store := NewStoreWithDB(db)

	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
			{ProviderID: "svc-2", Metadata: map[string]string{"serverId": "s-2"}},
		},
	}))

	// 中途删表后重注册（svc-2 消失 → 旧行清理分支；表缺失 → 删/upsert 均失败）。
	require.NoError(t, db.Migrator().DropTable("provider_metadata"))
	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-a",
		Env:     "dev",
		Providers: []ProviderSession{
			{ProviderID: "svc-1", Metadata: map[string]string{"serverId": "s-1"}},
		},
	}))
}
