package ops

// 受管程序更新边界物化（agent-supervisor-design §10.4 更新状态+指纹：审计）。
// 每次尝试（成功 / 回滚完成 / 客户端不可用）都写 execution_logs（source=
// process_update）——指纹硬要求规定持久查询主入口为本表，按「目标+时间+指纹」
// 检索；新旧指纹与回滚留痕必须入审计载荷。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func updateResp(success bool, rolledBack bool, oldFP, newFP, msg string) *opsv1.UpdateProcessResponse {
	return &opsv1.UpdateProcessResponse{
		Success:        success,
		Message:        msg,
		OldFingerprint: oldFP,
		NewFingerprint: newFP,
		RolledBack:     rolledBack,
	}
}

func TestOpsAgentProcessUpdateAudited(t *testing.T) {
	svcCtx, db := newExecAuditSvcCtx(t)
	store := registry.NewStore()
	for _, agentID := range []string{"agent-1", "agent-err"} {
		require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: agentID, GameID: "demo", Env: "prod"}))
	}
	svcCtx.RegistryStore = store
	ctx := context.Background()
	const (
		oldFP = "1111111111111111111111111111111111111111111111111111111111111111"
		newFP = "22222222222222222222222222222222222222222222222222222222222222222"
	)
	req := func() *OpsProcessUpdateRequest {
		return &OpsProcessUpdateRequest{AgentID: "agent-1", Name: "demo-app", StagedPath: "/tmp/staged", ExpectedSha256: newFP}
	}

	// 成功路径：指纹随响应落审计。
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgUpdateProcessRequest: updateResp(true, false, oldFP, newFP, "ok"),
	}})
	_, err := opsAgentProcessUpdate(ctx, svcCtx, req())
	require.NoError(t, err)

	// 回滚路径：wire 调用完成（RolledBack=true）同样留痕——回滚是业务结果。
	setExecCaller(t, "agent-1", &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgUpdateProcessRequest: updateResp(false, true, oldFP, newFP, "rolled back"),
	}})
	_, err = opsAgentProcessUpdate(ctx, svcCtx, req())
	require.NoError(t, err)

	// 客户端不可用路径：调用失败留 StatusFail，不留静默空洞。
	setExecCaller(t, "agent-err", &fakeCaller{errOn: map[uint32]error{
		protocol.MsgUpdateProcessRequest: assert.AnError,
	}})
	_, err = opsAgentProcessUpdate(ctx, svcCtx, &OpsProcessUpdateRequest{AgentID: "agent-err", Name: "demo-app", StagedPath: "/tmp/staged", ExpectedSha256: newFP})
	require.Error(t, err)

	// Writer.Stop 冲刷后逐条核对落库内容。
	svcCtx.ExecutionLogWriter.Stop()
	var logs []struct {
		Source         string
		FunctionID     string
		Route          string
		Status         string
		RequestPayload string
		ResponseBody   string
	}
	require.NoError(t, db.Table("execution_logs").Order("id").Find(&logs).Error)
	require.Len(t, logs, 3)

	byStatus := map[string]int{}
	for _, l := range logs {
		byStatus[l.Status]++
		assert.Equal(t, "process_update", l.Source, "更新审计有独立来源标识")
		assert.Equal(t, "demo-app", l.FunctionID, "进程名进审计主键位")
		assert.Contains(t, l.Route, "/ops/agents/", "路由留痕含 agent 与进程名")
		assert.Contains(t, l.Route, "/update")
		assert.Contains(t, l.RequestPayload, `"agentId"`, "请求载荷含 agentId")
		assert.Contains(t, l.RequestPayload, `"process"`, "请求载荷含进程名")
		assert.Contains(t, l.RequestPayload, `"stagedPath"`, "请求载荷含暂存路径")
		assert.Contains(t, l.RequestPayload, newFP, "期望指纹进请求载荷")
	}
	assert.Equal(t, 2, byStatus["ok"], "成功与回滚均为 wire 完成的调用")
	assert.Equal(t, 1, byStatus["error"], "wire 调用失败留 StatusFail")

	// 成功行响应载荷含旧→新指纹；回滚行含 rolledBack=true。
	var successRow, rollbackRow, failRow map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(logs[0].ResponseBody), &successRow))
	assert.Equal(t, true, successRow["success"])
	assert.Equal(t, oldFP, successRow["oldFingerprint"])
	assert.Equal(t, newFP, successRow["newFingerprint"])
	assert.Equal(t, false, successRow["rolledBack"])
	require.NoError(t, json.Unmarshal([]byte(logs[1].ResponseBody), &rollbackRow))
	assert.Equal(t, true, rollbackRow["rolledBack"], "回滚结果必须留痕")
	assert.Equal(t, newFP, rollbackRow["newFingerprint"])
	require.NoError(t, json.Unmarshal([]byte(logs[2].ResponseBody), &failRow))
	assert.Contains(t, failRow, "error", "失败行留错误详情")
}
