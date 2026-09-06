package ops

import (
	"context"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// invalidUTF8 让 proto.Marshal 校验失败（proto3 string 字段要求合法 UTF-8）。
const invalidUTF8 = "\xff\xfe"

// 覆盖 OpsClientWrapper 四个进程/命令方法的 proto.Marshal 失败分支。
func TestOpsClientWrapperMarshalFailuresV2(t *testing.T) {
	w := &OpsClientWrapper{caller: &mockSessionCaller{}}
	ctx := context.Background()

	if _, err := w.RestartProcess(ctx, &opsv1.RestartProcessRequest{ProcessName: invalidUTF8}); err == nil {
		t.Error("RestartProcess: expected marshal error")
	}
	if _, err := w.StopProcess(ctx, &opsv1.StopProcessRequest{ProcessName: invalidUTF8}); err == nil {
		t.Error("StopProcess: expected marshal error")
	}
	if _, err := w.StartProcess(ctx, &opsv1.StartProcessRequest{ProcessName: invalidUTF8}); err == nil {
		t.Error("StartProcess: expected marshal error")
	}
	if _, err := w.ExecuteCommand(ctx, &opsv1.ExecuteCommandRequest{Command: invalidUTF8}); err == nil {
		t.Error("ExecuteCommand: expected marshal error")
	}
}

// 覆盖 OpsAgentMetrics 的 since 过滤分支：指定 agent 的历史在 GetAgentMetrics
// 返回后仍需按 since 过滤，早于 since 的条目被跳过。
func TestOpsAgentMetricsSinceFilterV2(t *testing.T) {
	svcCtx := createTestServiceContext()
	metrics := svcCtx.MetricsStore
	metrics.Add("agent-a", &opsv1.MetricsReport{
		AgentId: "agent-a",
		Cpu:     &opsv1.CpuMetrics{Cores: 4},
		Memory:  &opsv1.MemoryMetrics{TotalBytes: 100},
	})
	// 等待 Received 时间戳落定（Add 使用 time.Now()）。
	time.Sleep(10 * time.Millisecond)

	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

	resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{AgentID: "agent-a", Since: future})
	if err != nil {
		t.Fatalf("OpsAgentMetrics: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected entries before since to be filtered, got %d", len(resp.Data))
	}

	// 不带 since 时同一条目应正常返回。
	resp, err = logic.OpsAgentMetrics(&OpsAgentMetricsRequest{AgentID: "agent-a"})
	if err != nil {
		t.Fatalf("OpsAgentMetrics without since: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 entry without since filter, got %d", len(resp.Data))
	}
}

// 覆盖 OpsAgentsList 对 nil session 的防御分支：map 中的 nil 会话被跳过。
func TestOpsAgentsListNilSessionV2(t *testing.T) {
	svcCtx := createTestServiceContext()
	store := svcCtx.RegistryStore
	store.Mu().Lock()
	store.AgentsUnsafe()["ghost-nil"] = nil
	store.Mu().Unlock()

	resp, err := NewOpsAgentsListLogic(context.Background(), svcCtx).OpsAgentsList(&OpsAgentsListRequest{})
	if err != nil {
		t.Fatalf("OpsAgentsList: %v", err)
	}
	for _, agent := range resp.Data {
		if agent.AgentID == "ghost-nil" {
			t.Fatal("nil session must be skipped")
		}
	}
}
