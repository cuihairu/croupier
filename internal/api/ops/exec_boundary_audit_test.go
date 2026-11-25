package ops

// Remote shell boundary materialization (agent-supervisor-design §10.7 trio
// prerequisite #2: audit). exec is a high-risk action: every attempt (success /
// execution failure / client unavailable) leaves an execution_logs record
// (§10.4 designates execlog as the primary audit entry point for authorized
// actions).

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/core/transport"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// newExecAuditSvcCtx wires an :memory: sqlite plus a real Writer (Run already
// started; Stop flushes and waits during Cleanup so assertions can check what
// landed in the database).
func newExecAuditSvcCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExecutionLog{}))
	writer := executionlog.NewWriter(db, executionlog.Config{})
	writer.Run(context.Background())
	t.Cleanup(writer.Stop)
	store := registry.NewStore()
	for _, agentID := range []string{"agent-1", "agent-err"} {
		require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: agentID, GameID: "demo", Env: "prod"}))
	}
	return &svc.ServiceContext{RegistryStore: store, ExecutionLogWriter: writer}, db
}

func setExecCaller(t *testing.T, agentID string, caller transport.SessionCaller) {
	t.Helper()
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{agentID: caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) }) // global singleton: reset after use
}

func execResp(exitCode int32, stdout, stderr string) *opsv1.ExecuteCommandResponse {
	return &opsv1.ExecuteCommandResponse{ExitCode: exitCode, StdOut: stdout, StdErr: stderr}
}

func TestOpsAgentExecCommandAudited(t *testing.T) {
	svcCtx, db := newExecAuditSvcCtx(t)
	ctx := context.Background()

	// success path: exit 0 leaves a trail.
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: execResp(0, "ok-out", ""),
	}})
	resp, err := opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls", Args: []string{"-l"}})
	require.NoError(t, err)
	assert.EqualValues(t, 0, resp.Result.ExitCode)

	// execution-failure path (non-zero exit) also leaves a trail — execution
	// completing with a non-zero exit code is a business failure.
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: execResp(2, "", "boom"),
	}})
	_, err = opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	require.NoError(t, err)

	// client-unavailable path: a call failure leaves a trail too, with no silent hole.
	setExecCaller(t, "agent-err", &fakeCaller{errOn: map[uint32]error{
		protocol.MsgExecuteCommandRequest: assert.AnError,
	}})
	_, err = opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-err", Command: "ls"})
	require.Error(t, err)

	// after Writer.Stop flushes, verify the landed content record by record.
	svcCtx.ExecutionLogWriter.Stop()
	var logs []model.ExecutionLog
	require.NoError(t, db.Order("id").Find(&logs).Error)
	require.Len(t, logs, 3)

	byStatus := map[string]int{}
	for _, l := range logs {
		byStatus[l.Status]++
		assert.Equal(t, executionlog.SourceAgentExec, l.Source, "exec 审计有独立来源标识")
		assert.Equal(t, "ls", l.FunctionID, "命令原文进审计主键位")
		assert.NotEmpty(t, l.RequestPayload, "审计载荷含请求原文（命令与参数）")
	}
	assert.Equal(t, 2, byStatus[executionlog.StatusOK], "两条执行完成（含 exit 2 的业务失败）")
	assert.Equal(t, 1, byStatus[executionlog.StatusFail], "wire 调用失败=执行失败留痕")
}

func TestOpsAgentExecCommandAuditNilWriterNoop(t *testing.T) {
	// A missing audit surface must not block the main execution path (legacy
	// assembly / no-Writer unit-test shape).
	ctx := context.Background()
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: execResp(0, "x", ""),
	}})

	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "agent-1", GameID: "demo", Env: "prod"}))
	resp, err := opsAgentExecCommand(ctx, &svc.ServiceContext{RegistryStore: store}, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	require.NoError(t, err)
	assert.EqualValues(t, 0, resp.Result.ExitCode)
}
