// Supervisor S1 server 侧测试：快照→DTO 映射、聚合灯闭集、API 读 MetricsStore
// 最新一报、agent 列表 supervisor 摘要注入、handler 路径参数兜底。
package ops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisorStateString(t *testing.T) {
	assert.Equal(t, "running", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_RUNNING))
	assert.Equal(t, "stopped", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_STOPPED))
	assert.Equal(t, "failed", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_FAILED))
	assert.Equal(t, "backoff", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_BACKOFF))
	assert.Equal(t, "broken", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_BROKEN))
	assert.Equal(t, "unknown", supervisorStateString(opsv1.ProcessState_PROCESS_STATE_UNSPECIFIED))
	assert.Equal(t, "unknown", supervisorStateString(opsv1.ProcessState(99)))
}

func TestSummarizeSupervised(t *testing.T) {
	// 空集 → ok
	assert.Equal(t, "ok", summarizeSupervised(nil).Status)
	assert.Equal(t, 0, summarizeSupervised(nil).Total)

	// 全 RUNNING 无标记 → ok
	ok := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		{State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
		{State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
	})
	assert.Equal(t, "ok", ok.Status)
	assert.Equal(t, 2, ok.Total)
	assert.Equal(t, 2, ok.Running)

	// 非 RUNNING → warn
	warn := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		{State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
		{State: opsv1.ProcessState_PROCESS_STATE_STOPPED},
	})
	assert.Equal(t, "warn", warn.Status)
	assert.Equal(t, 1, warn.Running)

	// 超限标记 → warn（即便 RUNNING）
	flagged := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		{State: opsv1.ProcessState_PROCESS_STATE_RUNNING, Flags: []string{"mem_over_limit"}},
	})
	assert.Equal(t, "warn", flagged.Status)

	// FAILED → error 优先
	bad := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		{State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
		{State: opsv1.ProcessState_PROCESS_STATE_FAILED},
	})
	assert.Equal(t, "error", bad.Status)

	// BROKEN → error
	broken := summarizeSupervised([]*opsv1.SupervisedProcessSnapshot{
		{State: opsv1.ProcessState_PROCESS_STATE_BROKEN},
	})
	assert.Equal(t, "error", broken.Status)
}

func TestSupervisedProcessFromSnapshot(t *testing.T) {
	out := supervisedProcessFromSnapshot(&opsv1.SupervisedProcessSnapshot{
		Name: "gameserver", Pid: 123, State: opsv1.ProcessState_PROCESS_STATE_RUNNING,
		UptimeSeconds: 60, RestartCount: 2, RssBytes: 1024, CpuPercent: 12.5,
		Flags: []string{"mem_over_limit"},
	})
	assert.Equal(t, "gameserver", out.Name)
	assert.Equal(t, "running", out.State)
	assert.Equal(t, int64(1024), out.RssBytes)
	assert.Equal(t, []string{"mem_over_limit"}, out.Flags)

	// 无 flags → 空切片而非 nil（JSON 序列化成 [] 而非 null）
	empty := supervisedProcessFromSnapshot(&opsv1.SupervisedProcessSnapshot{Name: "x"})
	assert.NotNil(t, empty.Flags)
	assert.Empty(t, empty.Flags)

	// S3：快照档透传
	snap := supervisedProcessFromSnapshot(&opsv1.SupervisedProcessSnapshot{Name: "g", SnapshotProfile: "jvm"})
	assert.Equal(t, "jvm", snap.SnapshotProfile)
}

func TestSupervisorEventFromProtoSnapshot(t *testing.T) {
	ev := supervisorEventFromProto(&opsv1.SupervisorEvent{
		Seq: 7, Process: "game", Event: "detect_down", ExitCode: 139, Signal: "SIGSEGV",
		SnapshotDir: "/snap/game", SnapshotFiles: []string{"dump.hprof", "notes.txt"},
	})
	assert.Equal(t, "detect_down", ev.Event)
	assert.Equal(t, "/snap/game", ev.SnapshotDir)
	assert.Equal(t, []string{"dump.hprof", "notes.txt"}, ev.SnapshotFiles)
}

func TestOpsAgentSupervisor_FromMetricsStore(t *testing.T) {
	store := registry.NewMetricsStore()
	report := &opsv1.MetricsReport{
		AgentId: "a1",
		SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
			{Name: "gameserver", Pid: 42, State: opsv1.ProcessState_PROCESS_STATE_RUNNING, RssBytes: 4096, CpuPercent: 3.5},
			{Name: "worker", State: opsv1.ProcessState_PROCESS_STATE_FAILED},
		},
	}
	store.Add("a1", report)

	svcCtx := &svc.ServiceContext{MetricsStore: store}
	resp, err := opsAgentSupervisor(context.Background(), svcCtx, &OpsAgentSupervisorRequest{AgentID: "a1"})
	require.NoError(t, err)
	assert.Equal(t, "a1", resp.AgentID)
	assert.NotEmpty(t, resp.Timestamp)
	require.Len(t, resp.Processes, 2)
	assert.Equal(t, "gameserver", resp.Processes[0].Name)
	assert.Equal(t, int64(4096), resp.Processes[0].RssBytes)
	assert.Equal(t, "failed", resp.Processes[1].State)
	assert.Equal(t, "error", resp.Summary.Status)
	assert.Equal(t, 2, resp.Summary.Total)
	assert.Equal(t, 1, resp.Summary.Running)

	// 未上报的 agent：空视图非错误（面板按 timestamp 判断新鲜度）
	resp2, err := opsAgentSupervisor(context.Background(), svcCtx, &OpsAgentSupervisorRequest{AgentID: "ghost"})
	require.NoError(t, err)
	assert.Empty(t, resp2.Processes)
	assert.Empty(t, resp2.Timestamp)
	assert.Equal(t, "ok", resp2.Summary.Status)
}

func TestOpsAgentSupervisor_NilStore(t *testing.T) {
	_, err := opsAgentSupervisor(context.Background(), &svc.ServiceContext{}, &OpsAgentSupervisorRequest{AgentID: "a1"})
	assert.Error(t, err)
	_, err = opsAgentSupervisor(context.Background(), nil, &OpsAgentSupervisorRequest{AgentID: "a1"})
	assert.Error(t, err)
}

func TestAttachSupervisorSummaries(t *testing.T) {
	store := registry.NewMetricsStore()
	store.Add("a1", &opsv1.MetricsReport{AgentId: "a1", SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
		{Name: "p", State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
	}})
	svcCtx := &svc.ServiceContext{MetricsStore: store}

	agents := []OpsAgentInfo{{AgentID: "a1"}, {AgentID: "a2"}}
	attachSupervisorSummaries(svcCtx, agents)
	require.NotNil(t, agents[0].Supervisor)
	assert.Equal(t, "ok", agents[0].Supervisor.Status)
	assert.Nil(t, agents[1].Supervisor, "no report → nil supervisor summary")

	// nil store / 空 agent 列表不 panic
	attachSupervisorSummaries(&svc.ServiceContext{}, agents)
	attachSupervisorSummaries(svcCtx, nil)
}

func TestOpsAgentSupervisorHandler_ParamFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := registry.NewMetricsStore()
	store.Add("a1", &opsv1.MetricsReport{AgentId: "a1", SupervisedProcesses: []*opsv1.SupervisedProcessSnapshot{
		{Name: "gameserver", Pid: 7, State: opsv1.ProcessState_PROCESS_STATE_RUNNING},
	}})
	svcCtx := &svc.ServiceContext{MetricsStore: store}
	handler := NewHandler(NewService(svcCtx))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ops/agents/a1/supervisor", nil)
	c.Params = gin.Params{{Key: "agentId", Value: "a1"}}
	handler.OpsAgentSupervisor(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body OpsAgentSupervisorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "a1", body.AgentID)
	require.Len(t, body.Processes, 1)
	assert.Equal(t, "gameserver", body.Processes[0].Name)
	assert.Equal(t, "running", body.Processes[0].State)
}
