package ops

// 远程 Shell 边界物化（agent-supervisor-design §10.7 三件套前置之二：审计）。
// exec 是高风险动作：每次尝试（成功 / 执行失败 / 客户端不可用）都留
// execution_logs 记录（§10.4 指定 execlog 为受授权动作审计主入口）。

import (
	"context"
	"testing"

	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// newExecAuditSvcCtx 装 :memory: sqlite + 真 Writer（Run 已启动，Cleanup 时
// Stop 冲刷并等待，供断言落库内容）。
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
	t.Cleanup(func() { client.SetSessionResolver(nil) }) // 全局单例：用后复位
}

func execResp(exitCode int32, stdout, stderr string) *opsv1.ExecuteCommandResponse {
	return &opsv1.ExecuteCommandResponse{ExitCode: exitCode, StdOut: stdout, StdErr: stderr}
}

func TestOpsAgentExecCommandAudited(t *testing.T) {
	svcCtx, db := newExecAuditSvcCtx(t)
	ctx := context.Background()

	// 成功路径：exit 0 留痕。
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: execResp(0, "ok-out", ""),
	}})
	resp, err := opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls", Args: []string{"-l"}})
	require.NoError(t, err)
	assert.EqualValues(t, 0, resp.Result.ExitCode)

	// 执行失败路径（exit 非 0）也留痕——执行完成但退出码非 0 是业务失败。
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgExecuteCommandRequest: execResp(2, "", "boom"),
	}})
	_, err = opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	require.NoError(t, err)

	// 客户端不可用路径：调用失败同样留痕，不留静默空洞。
	setExecCaller(t, "agent-err", &fakeCaller{errOn: map[uint32]error{
		protocol.MsgExecuteCommandRequest: assert.AnError,
	}})
	_, err = opsAgentExecCommand(ctx, svcCtx, &OpsExecCommandRequest{AgentID: "agent-err", Command: "ls"})
	require.Error(t, err)

	// Writer.Stop 冲刷后逐条核对落库内容。
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
	// 审计面缺失不得阻断执行主路径（旧装配 / 无 Writer 单元测试形态）。
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
