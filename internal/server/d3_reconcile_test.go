package server

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/common/labels"
	"github.com/cuihairu/croupier/internal/tasks"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-018 D3：agent 进程重启（同 agentId 的 bootId 变化）时，其在途任务
// （dispatching/running）落点不可知，必须收敛为 unknown 并追加 recovery 事件；
// 重连（同 bootId）或身份缺失不得误标。

func registerReq(agentID, gameID, env, bootID string) *agentv1.RegisterRequest {
	req := &agentv1.RegisterRequest{
		AgentId: agentID,
		GameId:  gameID,
		Env:     env,
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
	}
	if bootID != "" {
		req.Labels = map[string]string{labels.BootID: bootID}
	}
	return req
}

func TestHandleRegisterRequest_RestartBootIDChangeReconcilesRunningTasks(t *testing.T) {
	svc := newTestControlService()
	mock := &mockTaskStore{updateAccepted: true, runningIDs: []string{"task-1", "task-2"}}
	svc.SetTaskStore(mock)
	ctx := context.Background()

	// 第一次注册：boot-a
	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-rst", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)
	require.Equal(t, 0, mock.listRunningCalled, "first registration must not reconcile")

	// 第二次注册：boot-b（进程重启）→ 收敛在途任务
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-rst", "game-1", "dev", "boot-b"), "")
	require.NoError(t, err)

	assert.Equal(t, 1, mock.listRunningCalled)
	assert.Equal(t, "agent-rst", mock.lastListAgentID)
	assert.Equal(t, 2, mock.updateRunCalled, "both in-flight tasks reconciled")
	assert.Equal(t, tasks.StatusUnknown, mock.lastUpdates["status"])
	assert.NotNil(t, mock.lastUpdates["finished_at"])
	assert.Contains(t, mock.lastUpdates["error_message"], "boot-a -> boot-b")
	assert.Equal(t, 2, mock.appendCalled, "recovery event per reconciled task")
	assert.Equal(t, tasks.EventRecovery, mock.lastEventType)
}

func TestHandleRegisterRequest_ReconnectSameBootIDDoesNotReconcile(t *testing.T) {
	svc := newTestControlService()
	mock := &mockTaskStore{updateAccepted: true, runningIDs: []string{"task-1"}}
	svc.SetTaskStore(mock)
	ctx := context.Background()

	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-rec", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)
	// 断线重连：同一 bootId
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-rec", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)

	assert.Equal(t, 0, mock.listRunningCalled, "reconnect must not reconcile running tasks")
	assert.Equal(t, 0, mock.updateRunCalled)
	assert.Equal(t, 0, mock.appendCalled)
}

func TestHandleRegisterRequest_EmptyBootIDDoesNotReconcile(t *testing.T) {
	svc := newTestControlService()
	mock := &mockTaskStore{updateAccepted: true, runningIDs: []string{"task-1"}}
	svc.SetTaskStore(mock)
	ctx := context.Background()

	// 两轮都不带 bootId（旧 agent / 未声明重启身份）
	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-nb", "game-1", "dev", ""), "")
	require.NoError(t, err)
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-nb", "game-1", "dev", ""), "")
	require.NoError(t, err)

	assert.Equal(t, 0, mock.listRunningCalled, "missing bootId must not reconcile")
	assert.Equal(t, 0, mock.updateRunCalled)
}

func TestHandleRegisterRequest_RestartNoInFlightTasks(t *testing.T) {
	svc := newTestControlService()
	mock := &mockTaskStore{updateAccepted: true, runningIDs: nil}
	svc.SetTaskStore(mock)
	ctx := context.Background()

	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-idle", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-idle", "game-1", "dev", "boot-b"), "")
	require.NoError(t, err)

	assert.Equal(t, 1, mock.listRunningCalled)
	assert.Equal(t, 0, mock.updateRunCalled, "no in-flight tasks -> nothing to reconcile")
	assert.Equal(t, 0, mock.appendCalled)
}

func TestHandleRegisterRequest_RestartTerminalGuardSkipsAppend(t *testing.T) {
	svc := newTestControlService()
	// updateAccepted=false 模拟并发终态先到：终态单调，不回退、不追加 recovery。
	mock := &mockTaskStore{updateAccepted: false, runningIDs: []string{"task-1"}}
	svc.SetTaskStore(mock)
	ctx := context.Background()

	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-tm", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-tm", "game-1", "dev", "boot-b"), "")
	require.NoError(t, err)

	assert.Equal(t, 1, mock.updateRunCalled)
	assert.Equal(t, 0, mock.appendCalled, "terminal guard: no recovery event when update rejected")
}

func TestHandleRegisterRequest_RestartNilTaskStoreNoPanic(t *testing.T) {
	svc := newTestControlService() // taskStore 未设置（nil）
	ctx := context.Background()

	_, err := svc.handleRegisterRequest(ctx, registerReq("agent-nil", "game-1", "dev", "boot-a"), "")
	require.NoError(t, err)
	// 重启但 taskStore nil：不 panic、注册不失败，仅跳过对账。
	_, err = svc.handleRegisterRequest(ctx, registerReq("agent-nil", "game-1", "dev", "boot-b"), "")
	require.NoError(t, err)
}

func TestSanitizeAgentLabelsKeepsBootID(t *testing.T) {
	in := map[string]string{
		labels.BootID:            "boot-1",
		labels.AgentBinarySHA256: "abc123",
		"os":                     "linux",
	}
	out := sanitizeAgentLabels(in)
	assert.Equal(t, "boot-1", out[labels.BootID])
	assert.Equal(t, "abc123", out[labels.AgentBinarySHA256])
	assert.Equal(t, "linux", out["os"])
}
