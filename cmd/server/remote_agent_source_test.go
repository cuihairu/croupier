package main

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ownerAgentSource 契约：只供应对端实例持有的 agent（本实例自有的走本地
// registry，不重复），scope 匹配 + 快照表有活跃行；归属表/快照表的竞态
// 窗口（owner 已 Release、行缺失）静默跳过。

func newRemoteSourceFixture(t *testing.T) (*ownerAgentSource, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, reg.MigrateAgentSessions(db))
	return &ownerAgentSource{db: db, selfID: "server1"}, db
}

func ownersFunc(recs ...cluster.AgentOwnerRecord) func(context.Context) ([]cluster.AgentOwnerRecord, error) {
	return func(context.Context) ([]cluster.AgentOwnerRecord, error) { return recs, nil }
}

// 对端持有 + 快照表活跃 → 供应；本实例自有 → 排除（本地 registry 负责）。
func TestOwnerAgentSource_ExcludesSelfOwned(t *testing.T) {
	src, db := newRemoteSourceFixture(t)
	require.NoError(t, reg.NewAgentSessionModel(db).Upsert(context.Background(), refreshDBSession("agent-peer")))
	require.NoError(t, reg.NewAgentSessionModel(db).Upsert(context.Background(), refreshDBSession("agent-self")))
	src.owners = ownersFunc(
		cluster.AgentOwnerRecord{AgentID: "agent-peer", InstanceID: "server2", GameID: "demo", Env: "prod"},
		cluster.AgentOwnerRecord{AgentID: "agent-self", InstanceID: "server1", GameID: "demo", Env: "prod"},
	)

	sessions, err := src.RemoteAgentSessions(context.Background(), "demo", "prod", true)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "agent-peer", sessions[0].AgentID)
}

// scoped 时归属记录的 scope 不匹配 → 不查快照表。
func TestOwnerAgentSource_ScopeFilter(t *testing.T) {
	src, _ := newRemoteSourceFixture(t)
	src.owners = ownersFunc(
		cluster.AgentOwnerRecord{AgentID: "agent-peer", InstanceID: "server2", GameID: "other", Env: "prod"},
	)

	sessions, err := src.RemoteAgentSessions(context.Background(), "demo", "prod", true)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// 归属表活跃但快照表无行（owner 已 Release 的竞态）→ 跳过，不报错。
func TestOwnerAgentSource_MissingSnapshotRowSkipped(t *testing.T) {
	src, _ := newRemoteSourceFixture(t)
	src.owners = ownersFunc(
		cluster.AgentOwnerRecord{AgentID: "agent-ghost", InstanceID: "server2", GameID: "demo", Env: "prod"},
	)

	sessions, err := src.RemoteAgentSessions(context.Background(), "demo", "prod", true)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// 快照行还原函数注册表（Functions/Providers JSON 落库往返），dispatcher
// 侧 agentCanInvoke/targeted 过滤可用。
func TestOwnerAgentSource_SnapshotCarriesFunctionsAndProviders(t *testing.T) {
	src, db := newRemoteSourceFixture(t)
	require.NoError(t, reg.NewAgentSessionModel(db).Upsert(context.Background(), refreshDBSession("agent-peer", "svc-1")))
	src.owners = ownersFunc(
		cluster.AgentOwnerRecord{AgentID: "agent-peer", InstanceID: "server2", GameID: "demo", Env: "prod"},
	)

	sessions, err := src.RemoteAgentSessions(context.Background(), "demo", "prod", true)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Contains(t, sessions[0].Functions, "demo.fn")
	assert.True(t, sessions[0].Functions["demo.fn"].Enabled)
	require.Len(t, sessions[0].Providers, 1)
	assert.Equal(t, "svc-1", sessions[0].Providers[0].ProviderID)
}
