package ops

// supervisor.go 补测：supervisorStateString 的 STARTING/STOPPING 臂、
// summarizeSupervised 与 agentSupervisorFromEntry 的 nil 快照跳过、两个
// supervisor 视图函数的 metrics store 未装配守卫、supervisor 日志拉取的
// 默认 tail 上限与往返面。
//
// 诚实缺口：opsAgentSupervisorLog 的 svcCtx==nil 守卫不可达——requireAgentScope
// 先于它检查 svcCtx 并报 "registry store unavailable"，无输入能穿过前者而
// 保持 svcCtx 为 nil。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestSupervisorStateStringStartingStopping(t *testing.T) {
	require.Equal(t, "starting", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_STARTING))
	require.Equal(t, "stopping", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_STOPPING))
	require.Equal(t, "unknown", supervisorStateString(opsv1.ProcessState(999)))
}

func TestSummarizeSupervisedSkipsNilSnapshot(t *testing.T) {
	// nil 元素跳过，不 panic；剩余全 RUNNING 无标记 → ok
	s := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		nil,
		{Name: "game", State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
	})
	require.Equal(t, "ok", s.Status)
	require.Equal(t, 1, s.Running)
}

func TestAgentSupervisorFromEntrySkipsNilSnapshot(t *testing.T) {
	resp := agentSupervisorFromEntry("a1", &registry.MetricsEntry{
		Report: &opsv1.MetricsReport{
			SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
				nil,
				{Name: "game", State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
			},
		},
	})
	require.Len(t, resp.Processes, 1)
	require.Equal(t, "game", resp.Processes[0].Name)
}

func TestOpsAgentSupervisorMetricsStoreGuards(t *testing.T) {
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	_, err := opsAgentSupervisor(ctx, svcCtx, &OpsAgentSupervisorRequest{AgentID: "a1"})
	require.ErrorContains(t, err, "metrics store unavailable")

	_, err = opsAgentSupervisorEvents(ctx, svcCtx, &OpsAgentSupervisorEventsRequest{AgentID: "a1"})
	require.ErrorContains(t, err, "metrics store unavailable")
}

func TestOpsAgentSupervisorLogBranches(t *testing.T) {
	// GetClient 失败（resolver 未装配）→ "ops client unavailable"
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	opsGetClient().SetSessionResolver(nil)
	_, err := opsAgentSupervisorLog(ctx, &svc.ServiceContext{RegistryStore: store}, &OpsAgentSupervisorLogRequest{AgentID: "a1"})
	require.ErrorContains(t, err, "ops client unavailable")

	// 往返面：MaxBytes<=0 → 默认 tail；罐头内容原样返回
	caller := &fakeCaller{respFor: map[uint32]proto.Message{
		protocol.MsgGetSupervisorLogRequest: &opsv1.GetSupervisorLogResponse{Content: []byte("line1\n"), Truncated: true},
	}}
	setupOpsResolver(t, caller)
	got, err := opsAgentSupervisorLog(ctx, &svc.ServiceContext{RegistryStore: store}, &OpsAgentSupervisorLogRequest{AgentID: "a1", MaxBytes: 0})
	require.NoError(t, err)
	require.Equal(t, "line1\n", string(got.Content))
	require.True(t, got.Truncated)
	var sent opsv1.GetSupervisorLogRequest
	require.NoError(t, proto.Unmarshal(caller.lastBody[uint32(protocol.MsgGetSupervisorLogRequest)], &sent))
	require.EqualValues(t, defaultSupervisorLogMaxBytes, sent.MaxBytes)
}
