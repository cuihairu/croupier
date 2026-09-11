package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pruneTestSession(agentID string, lastSeen time.Time) *AgentSession {
	return &AgentSession{
		AgentID:   agentID,
		GameID:    "demo",
		Env:       "prod",
		ExpireAt:  lastSeen.Add(2 * time.Hour),
		LastSeen:  lastSeen,
		Functions: map[string]FunctionMeta{"demo.fn": {Enabled: true}},
	}
}

// 陈旧条目（LastSeen 早于 notAfter）删除成功。
func TestRemoveAgentIfStale_StaleRemoved(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.UpsertAgent(pruneTestSession("agent-x", time.Now().Add(-10*time.Minute))))

	removed := store.RemoveAgentIfStale("agent-x", time.Now())

	assert.True(t, removed)
	store.Mu().RLock()
	_, exists := store.AgentsUnsafe()["agent-x"]
	store.Mu().RUnlock()
	assert.False(t, exists, "断连清理后条目必须立即消失（不再僵尸到 ExpireAt）")
}

// 新鲜条目（notAfter 之后有注册/心跳刷新 LastSeen）不得删除——断连清理
// 与瞬间重连注册的竞态防护。
func TestRemoveAgentIfStale_FreshKept(t *testing.T) {
	store := NewStore()
	fresh := time.Now()
	require.NoError(t, store.UpsertAgent(pruneTestSession("agent-x", fresh)))

	// 断连时刻早于新注册的 LastSeen：注册发生在断连清理开始之后。
	removed := store.RemoveAgentIfStale("agent-x", fresh.Add(-time.Second))

	assert.False(t, removed)
	store.Mu().RLock()
	_, exists := store.AgentsUnsafe()["agent-x"]
	store.Mu().RUnlock()
	assert.True(t, exists, "断连瞬间重连注册的新会话不得被旧连接清理误删")
}

// 不存在的条目/空 agentID 返回 false，不 panic。
func TestRemoveAgentIfStale_MissingNoop(t *testing.T) {
	store := NewStore()
	assert.False(t, store.RemoveAgentIfStale("agent-missing", time.Now().Add(-time.Hour)))
	assert.False(t, store.RemoveAgentIfStale("", time.Now()))
}

// keep 过滤：归属表活跃集合之外的快照行不灌回。
func TestLoadFromDBFiltered_KeepFiltersZombieRows(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	now := time.Now()
	require.NoError(t, model.Upsert(context.Background(), pruneTestSession("agent-alive", now)))
	require.NoError(t, model.Upsert(context.Background(), pruneTestSession("agent-dead", now)))

	store := NewStoreWithDB(db)
	alive := map[string]bool{"agent-alive": true}
	err := store.LoadFromDBFiltered(context.Background(), model, func(sess *AgentSession) bool {
		return alive[sess.AgentID]
	})
	require.NoError(t, err)

	store.Mu().RLock()
	agents := store.AgentsUnsafe()
	store.Mu().RUnlock()
	assert.NotNil(t, agents["agent-alive"], "归属表活跃的快照行必须恢复")
	assert.Nil(t, agents["agent-dead"], "归属表无行的残留快照行不得复活")
}

// keep == nil 等价既有 LoadFromDB 全量恢复（单实例语义不变）。
func TestLoadFromDBFiltered_NilKeepsAll(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	now := time.Now()
	require.NoError(t, model.Upsert(context.Background(), pruneTestSession("agent-a", now)))
	require.NoError(t, model.Upsert(context.Background(), pruneTestSession("agent-b", now)))

	store := NewStoreWithDB(db)
	require.NoError(t, store.LoadFromDBFiltered(context.Background(), model, nil))

	store.Mu().RLock()
	count := len(store.AgentsUnsafe())
	store.Mu().RUnlock()
	assert.Equal(t, 2, count, "nil filter 全量恢复")
}
