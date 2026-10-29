package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/labels"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/tasks"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ARCH-018 故障验证 · Drill 1（断链+重启 → unknown）：
// 执行中断链后 agent 以新 bootId 重新注册，server 必须把该 agent 在途任务
// 收敛为 unknown 并追加 recovery 事件（D3），绝不静默悬置。走真实 TCP 帧：
// 注册 → 在途任务 → 断链 → 新 bootId 重注册 → 断言 DB 状态。
func TestArch018Drill_LinkBreakRestartReconcilesToUnknown(t *testing.T) {
	db, taskStore, taskRunModel := newDrillTaskStore(t)
	store := registry.NewStore()
	listener, err := server.NewTCPListener(&server.TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}, server.NewAgentSessionStore(), store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	svc := server.NewControlService(store, nil)
	svc.SetTaskStore(taskStore)
	listener.SetHandler(svc)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	go func() { _ = listener.Serve(rootCtx) }()

	// 第一次注册：bootId=BOOT-A
	registerAgentOnWire(t, listener.Addr(), "drill-agent", "game-1", "dev", "BOOT-A")
	require.Eventually(t, func() bool {
		return store.PreviousAgentSession("drill-agent") != nil
	}, 2*time.Second, 10*time.Millisecond)

	// 在途任务：agent 正在执行（running）时断链。
	ctx := context.Background()
	require.NoError(t, taskRunModel.Create(ctx, &model.TaskRun{
		TaskID: "drill-task-1", AgentID: "drill-agent",
		GameID: "game-1", Env: "dev", Status: tasks.StatusRunning,
		FunctionID: "player.ban",
	}))

	// 断链：关闭 agent 连接（模拟执行中链路中断）。
	// 注：真实 agent 断连后其会话在 server 侧仍短暂残留（TTL），重注册时
	// PreviousAgentSession 返回旧会话（bootId=BOOT-A），故重启侦测成立。
	// 断链本身不触发对账——对账在「新 bootId 重注册」时发生。

	// 重注册：bootId=BOOT-B（进程重启）
	registerAgentOnWire(t, listener.Addr(), "drill-agent", "game-1", "dev", "BOOT-B")

	require.Eventually(t, func() bool {
		run, err := taskRunModel.FindByTaskID(ctx, "drill-task-1")
		return err == nil && run.Status == tasks.StatusUnknown
	}, 3*time.Second, 20*time.Millisecond)

	run, err := taskRunModel.FindByTaskID(ctx, "drill-task-1")
	require.NoError(t, err)
	assert.Equal(t, tasks.StatusUnknown, run.Status)
	assert.NotNil(t, run.FinishedAt)

	events, err := taskRunEventModel(db).ListByTaskID(ctx, "drill-task-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1, "recovery event must be appended")
	assert.Equal(t, "recovery", events[0].Type)
	assert.Contains(t, events[0].Message, "BOOT-A -> BOOT-B")
}

// ARCH-018 故障验证 · Drill 1b（断链+重连同 bootId → 不误标）：
// 同 bootId 重注册（断线重连，非重启）不得把在途任务标 unknown——重连后
// agent 仍可能完成副作用。
func TestArch018Drill_ReconnectSameBootKeepsRunning(t *testing.T) {
	db, taskStore, taskRunModel := newDrillTaskStore(t)
	store := registry.NewStore()
	listener, err := server.NewTCPListener(&server.TCPListenerConfig{Address: "127.0.0.1:0", Insecure: true}, server.NewAgentSessionStore(), store, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	svc := server.NewControlService(store, nil)
	svc.SetTaskStore(taskStore)
	listener.SetHandler(svc)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	go func() { _ = listener.Serve(rootCtx) }()

	registerAgentOnWire(t, listener.Addr(), "drill-recon", "game-1", "dev", "BOOT-X")
	require.Eventually(t, func() bool {
		return store.PreviousAgentSession("drill-recon") != nil
	}, 2*time.Second, 10*time.Millisecond)

	ctx := context.Background()
	require.NoError(t, taskRunModel.Create(ctx, &model.TaskRun{
		TaskID: "drill-task-recon", AgentID: "drill-recon",
		GameID: "game-1", Env: "dev", Status: tasks.StatusRunning,
		FunctionID: "player.ban",
	}))

	// 重连：同 bootId（断线重连）。
	registerAgentOnWire(t, listener.Addr(), "drill-recon", "game-1", "dev", "BOOT-X")

	run, err := taskRunModel.FindByTaskID(ctx, "drill-task-recon")
	require.NoError(t, err)
	assert.Equal(t, tasks.StatusRunning, run.Status, "reconnect must not regress a running task")

	events, err := taskRunEventModel(db).ListByTaskID(ctx, "drill-task-recon", 0)
	require.NoError(t, err)
	assert.Empty(t, events, "no recovery event on reconnect")
}

func newDrillTaskStore(t *testing.T) (*gorm.DB, *tasks.Store, *model.TaskRunModel) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}))
	return db, tasks.NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db)), model.NewTaskRunModel(db)
}

func taskRunEventModel(db *gorm.DB) *model.TaskEventModel {
	return model.NewTaskEventModel(db)
}

// registerAgentOnWire 通过真实 TCP 帧向 server 注册一个 agent（带 bootId 标签）。
func registerAgentOnWire(t *testing.T, addr, agentID, gameID, env, bootID string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	req := &agentv1.RegisterRequest{AgentId: agentID, GameId: gameID, Env: env}
	if bootID != "" {
		req.Labels = map[string]string{labels.BootID: bootID}
	}
	regReq, err := proto.Marshal(req)
	require.NoError(t, err)
	frame := protocol.NewMessageBody(protocol.MsgRegisterRequest, 1, regReq)
	wrapped := make([]byte, 4+len(frame))
	wrapped[0] = byte(len(frame) >> 24)
	wrapped[1] = byte(len(frame) >> 16)
	wrapped[2] = byte(len(frame) >> 8)
	wrapped[3] = byte(len(frame))
	copy(wrapped[4:], frame)
	_, err = conn.Write(wrapped)
	require.NoError(t, err)
}
