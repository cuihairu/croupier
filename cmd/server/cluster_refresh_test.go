package main

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// refreshOwnerFake 只驱动 ListAliveOwners（refreshRemoteSnapshots 的唯一
// 归属表入口），其余方法 no-op。
type refreshOwnerFake struct {
	recs []cluster.AgentOwnerRecord
}

func (f *refreshOwnerFake) ClaimOwner(context.Context, string, string, string, string, uint64) error {
	return nil
}
func (f *refreshOwnerFake) Touch(context.Context, string) error   { return nil }
func (f *refreshOwnerFake) Release(context.Context, string) error { return nil }
func (f *refreshOwnerFake) ResolveOwner(context.Context, string) (*cluster.PeerInfo, error) {
	return nil, nil
}
func (f *refreshOwnerFake) FindOwner(context.Context, string) (*cluster.AgentOwnerRecord, error) {
	return nil, nil
}
func (f *refreshOwnerFake) ListAliveOwners(context.Context) ([]cluster.AgentOwnerRecord, error) {
	return f.recs, nil
}
func (f *refreshOwnerFake) CountAgentsByOwner(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (f *refreshOwnerFake) SelfOwnerScope(context.Context, string) (string, string, bool) {
	return "", "", false
}
func (f *refreshOwnerFake) SetMesh(*cluster.MeshInterconnect) {}

// newRefreshFixture：内存 registry store（本地快照）+ 独立 sqlite（共享
// agent_sessions 快照表，DB 侧真相）。
func newRefreshFixture(t *testing.T) (*svc.ServiceContext, *reg.Store) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	store := reg.NewStore()
	return &svc.ServiceContext{DB: db, RegistryStore: store}, store
}

func refreshDBSession(agentID string, providerIDs ...string) *reg.AgentSession {
	now := time.Now()
	providers := make([]reg.ProviderSession, 0, len(providerIDs))
	for _, id := range providerIDs {
		providers = append(providers, reg.ProviderSession{
			ProviderID:  id,
			GameID:      "demo",
			Env:         "prod",
			Addr:        "10.0.0.2:40252",
			FunctionIDs: []string{"demo.fn"},
		})
	}
	return &reg.AgentSession{
		AgentID:   agentID,
		GameID:    "demo",
		Env:       "prod",
		Functions: map[string]reg.FunctionMeta{"demo.fn": {Enabled: true}},
		Providers: providers,
		ExpireAt:  now.Add(5 * time.Minute),
		LastSeen:  now,
	}
}

func localProviderIDs(t *testing.T, store *reg.Store, agentID string) []string {
	t.Helper()
	store.Mu().RLock()
	defer store.Mu().RUnlock()
	sess, ok := store.AgentsUnsafe()[agentID]
	if !ok || sess == nil {
		return nil
	}
	ids := make([]string, 0, len(sess.Providers))
	for _, p := range sess.Providers {
		ids = append(ids, p.ProviderID)
	}
	return ids
}

// 线上缺陷回归：agent 函数表数量不变、Provider 增加（第二个 provider 注册）
// 时，旧条件 len(Functions) 比较不触发回灌，对端实例的本地快照 Providers
// 永久落后（/functions/instances 在对端少 19 条的根因）。
func TestRefreshRemoteSnapshots_ProviderGrowthTriggersRefresh(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, reg.MigrateAgentSessions(svcCtx.DB))
	// DB 真相：1 函数 + 2 provider；本地快照：同 1 函数 + 1 provider，
	// ExpireAt 远期（排除临期条件触发，锁定 providers 比较本身）。
	require.NoError(t, reg.NewAgentSessionModel(svcCtx.DB).Upsert(context.Background(), refreshDBSession("agent-x", "p1", "p2")))
	local := refreshDBSession("agent-x", "p1")
	local.ExpireAt = time.Now().Add(2 * time.Hour)
	require.NoError(t, store.UpsertAgent(local))

	refreshRemoteSnapshots(context.Background(), svcCtx, &refreshOwnerFake{
		recs: []cluster.AgentOwnerRecord{{AgentID: "agent-x", InstanceID: "server2", GameID: "demo", Env: "prod"}},
	}, "server1")

	assert.ElementsMatch(t, []string{"p1", "p2"}, localProviderIDs(t, store, "agent-x"),
		"Provider 增加必须触发快照回灌（函数表不变时亦然）")
}

// 负向对照：函数表与 Provider 数量都不落后（仅明细差异）时不回灌，避免
// 30s 周期内用 DB 节流窗口的旧明细覆盖本地。
func TestRefreshRemoteSnapshots_ProviderCountEqual_NoRefresh(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, reg.MigrateAgentSessions(svcCtx.DB))
	require.NoError(t, reg.NewAgentSessionModel(svcCtx.DB).Upsert(context.Background(), refreshDBSession("agent-x", "p-db")))
	local := refreshDBSession("agent-x", "p-local")
	local.ExpireAt = time.Now().Add(2 * time.Hour)
	require.NoError(t, store.UpsertAgent(local))

	refreshRemoteSnapshots(context.Background(), svcCtx, &refreshOwnerFake{
		recs: []cluster.AgentOwnerRecord{{AgentID: "agent-x", InstanceID: "server2", GameID: "demo", Env: "prod"}},
	}, "server1")

	assert.Equal(t, []string{"p-local"}, localProviderIDs(t, store, "agent-x"),
		"数量不落后时不得覆盖本地快照")
}

// 本实例自持的 agent（owner=self）不进回灌集合，本地实时会话不被 DB 行覆盖。
func TestRefreshRemoteSnapshots_SelfOwnedAgentUntouched(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, reg.MigrateAgentSessions(svcCtx.DB))
	require.NoError(t, reg.NewAgentSessionModel(svcCtx.DB).Upsert(context.Background(), refreshDBSession("agent-x", "p1", "p2")))
	local := refreshDBSession("agent-x", "p1")
	require.NoError(t, store.UpsertAgent(local))

	refreshRemoteSnapshots(context.Background(), svcCtx, &refreshOwnerFake{
		recs: []cluster.AgentOwnerRecord{{AgentID: "agent-x", InstanceID: "server1", GameID: "demo", Env: "prod"}},
	}, "server1")

	assert.Equal(t, []string{"p1"}, localProviderIDs(t, store, "agent-x"))
}
