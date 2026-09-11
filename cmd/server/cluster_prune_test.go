package main

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pruneLocalSession(agentID string, lastSeen time.Time) *reg.AgentSession {
	sess := refreshDBSession(agentID, "p1")
	sess.LastSeen = lastSeen
	sess.ExpireAt = lastSeen.Add(2 * time.Hour)
	return sess
}

func pruneHasAgent(store *reg.Store, agentID string) bool {
	store.Mu().RLock()
	defer store.Mu().RUnlock()
	_, ok := store.AgentsUnsafe()[agentID]
	return ok
}

// 归属表无行（无任何实例持有连接）且 LastSeen 超过宽限窗口 → 清理。
func TestPruneOrphanSnapshots_NoOwnerStaleRemoved(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, store.UpsertAgent(pruneLocalSession("agent-dead", time.Now().Add(-6*time.Minute))))

	pruneOrphanSnapshots(svcCtx, nil)

	assert.False(t, pruneHasAgent(store, "agent-dead"),
		"归属表无行 + LastSeen 陈旧的条目必须清理（不再僵尸到 ExpireAt 24h）")
}

// 归属表有行（任一实例持有连接）→ 即便本地 LastSeen 陈旧也不清：归属表
// 是存活主判据，快照副本的 LastSeen 从不新鲜（对端持有时心跳不打本地）。
func TestPruneOrphanSnapshots_AliveOwnerKept(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, store.UpsertAgent(pruneLocalSession("agent-alive", time.Now().Add(-30*time.Minute))))

	pruneOrphanSnapshots(svcCtx, []cluster.AgentOwnerRecord{
		{AgentID: "agent-alive", InstanceID: "server2"},
	})

	assert.True(t, pruneHasAgent(store, "agent-alive"),
		"归属表活跃的 agent 不得因本地 LastSeen 陈旧被误删")
}

// 归属表无行但 LastSeen 新鲜（宽限窗口内）→ 不清：防 Touch 失败/DB 抖动
// 导致归属行瞬时缺失但连接活着的误删（活连接心跳直接刷内存 LastSeen）。
func TestPruneOrphanSnapshots_FreshLastSeenKept(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, store.UpsertAgent(pruneLocalSession("agent-fresh", time.Now())))

	pruneOrphanSnapshots(svcCtx, nil)

	assert.True(t, pruneHasAgent(store, "agent-fresh"),
		"宽限窗口内的条目不得清理（归属行可能只是瞬时缺失）")
}

// refreshRemoteSnapshots 全链对账：owners 为空（归属表干净）时本地陈旧
// 条目也被清理——提前 return 不再跳过清理分支。
func TestRefreshRemoteSnapshots_OrphanPrunedViaRefresh(t *testing.T) {
	svcCtx, store := newRefreshFixture(t)
	require.NoError(t, store.UpsertAgent(pruneLocalSession("agent-zombie", time.Now().Add(-10*time.Minute))))

	refreshRemoteSnapshots(context.Background(), svcCtx, &refreshOwnerFake{recs: nil}, "server1")

	assert.False(t, pruneHasAgent(store, "agent-zombie"),
		"归属表空的周期对账也要清理本地僵尸")
}
