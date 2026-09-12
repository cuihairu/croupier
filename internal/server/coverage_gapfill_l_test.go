package server

// 覆盖率补洞（group L）：control_handler.go snapshotAgentSession 的 nil 与
// Labels 深拷贝分支；handleHeartbeatRequest 自愈 reseed 的 UpsertAgent 失败
// 分支（关闭底层 sql.DB 注入确定性错误）。只新增用例，不改产品代码与既有测试。

import (
	"context"
	"log/slog"
	"testing"

	gsqlite "github.com/glebarez/sqlite"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// lCovOwnerLookup 声明本实例持有目标 agent（驱动心跳自愈 reseed 路径）。
type lCovOwnerLookup struct{}

func (lCovOwnerLookup) SelfOwnerScope(context.Context, string) (string, string, bool) {
	return "demo_game", "development", true
}

// TestSnapshotAgentSession_Nil：nil 会话直接返回 nil。
func TestSnapshotAgentSession_Nil(t *testing.T) {
	assert.Nil(t, snapshotAgentSession(nil))
}

// TestSnapshotAgentSession_DeepCopiesLabels：Labels 是原地 merge 语义，快照
// 必须深拷贝；无 Labels 会话的快照 Labels 保持 nil。
func TestSnapshotAgentSession_DeepCopiesLabels(t *testing.T) {
	src := &registry.AgentSession{
		AgentID:   "agent-1",
		GameID:    "demo_game",
		Env:       "development",
		Labels:    map[string]string{"reportedOwner": "inst-1"},
		Functions: map[string]registry.FunctionMeta{"player.query": {OperationID: "player.query", Version: "1.0.0"}},
	}
	snap := snapshotAgentSession(src)
	require.NotNil(t, snap)

	src.Labels["reportedOwner"] = "mutated"
	assert.Equal(t, "inst-1", snap.Labels["reportedOwner"], "Labels must be deep-copied")
	assert.Equal(t, len(src.Functions), len(snap.Functions))

	plain := snapshotAgentSession(&registry.AgentSession{AgentID: "agent-2"})
	require.NotNil(t, plain)
	assert.Nil(t, plain.Labels)
}

// TestHandleHeartbeatRequest_SelfHealReseedUpsertFails：自愈 reseed 写回
// 失败（底层 sql.DB 已关闭，Transaction 确定性报错）时只记日志并返回空
// 应答——不向 agent 泄漏内部错误，也不留下半成品会话。
func TestHandleHeartbeatRequest_SelfHealReseedUpsertFails(t *testing.T) {
	gdb, err := gorm.Open(gsqlite.Open(t.TempDir()+"/l_cov_reseed.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close()) // 后续 Transaction 必败：注入 UpsertAgent 错误

	svc := NewControlService(registry.NewStoreWithDB(gdb), nil)
	svc.SetLogger(slog.Default())
	svc.SetHeartbeatOwnerLookup(lCovOwnerLookup{})

	resp, err := svc.handleHeartbeatRequest(context.Background(), &agentv1.HeartbeatRequest{AgentId: "agent-ghost"})
	require.NoError(t, err, "reseed failure must not surface as a heartbeat error")
	require.NotNil(t, resp)

	svc.registry.Mu().RLock()
	_, present := svc.registry.AgentsUnsafe()["agent-ghost"]
	svc.registry.Mu().RUnlock()
	assert.False(t, present, "failed reseed must not leave a session behind")
}
