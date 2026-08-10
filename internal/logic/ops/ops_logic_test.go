package ops

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// OpsAgentsList
// ---------------------------------------------------------------------------

func TestOpsAgentsList_NilStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: nil}
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestOpsAgentsList_EmptyStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{RegistryStore: registry.NewStore()}
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestOpsAgentsList_WithAgents(t *testing.T) {
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-1",
		GameID:   "demo",
		Env:      "prod",
		Version:  "v1.0.0",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
			"func2": {Enabled: true},
		},
		Labels: map[string]string{"dc": "dc1"},
	})
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-2",
		GameID:   "demo",
		Env:      "staging",
		ExpireAt: time.Now().Add(-time.Minute), // expired
		LastSeen: time.Now().Add(-2 * time.Minute),
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Len(t, resp.Data, 2)

	// Find agent-1
	var a1 OpsAgentInfo
	for _, a := range resp.Data {
		if a.AgentID == "agent-1" {
			a1 = a
		}
	}
	assert.Equal(t, "demo", a1.GameID)
	assert.Equal(t, "prod", a1.Env)
	assert.True(t, a1.Connected)
	assert.Len(t, a1.Functions, 2)
	assert.Equal(t, map[string]string{"dc": "dc1"}, a1.Labels)
}

// ---------------------------------------------------------------------------
// OpsAgentMetrics
// ---------------------------------------------------------------------------

func TestOpsAgentMetrics_NilStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{MetricsStore: nil}
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestOpsAgentMetrics_EmptyStore(t *testing.T) {
	svcCtx := &svc.ServiceContext{MetricsStore: registry.NewMetricsStore()}
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

func TestOpsAgentMetrics_WithEntries(t *testing.T) {
	store := registry.NewMetricsStore()
	store.Add("agent-1", &opsv1.MetricsReport{
		Cpu: &opsv1.CpuMetrics{Cores: 4, UsagePercent: 50.0, Load_1M: 1.5},
		Memory: &opsv1.MemoryMetrics{
			TotalBytes:   8 * 1024 * 1024 * 1024,
			UsedBytes:    4 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
		Disks: []*opsv1.DiskMetrics{
			{MountPoint: "/", Device: "/dev/sda1", TotalBytes: 100 * 1024 * 1024 * 1024, UsagePercent: 60.0},
		},
		Networks: []*opsv1.NetworkMetrics{
			{Interface: "eth0", BytesSent: 1024, BytesRecv: 2048},
		},
	})

	svcCtx := &svc.ServiceContext{MetricsStore: store}
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

	t.Run("by agent ID", func(t *testing.T) {
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{AgentID: "agent-1", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "agent-1", resp.Data[0].AgentID)
		assert.Equal(t, int32(4), resp.Data[0].CPU.Cores)
		assert.Equal(t, 50.0, resp.Data[0].CPU.UsagePercent)
		assert.Len(t, resp.Data[0].Disks, 1)
		assert.Len(t, resp.Data[0].Networks, 1)
	})

	t.Run("all metrics", func(t *testing.T) {
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{Limit: 100})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
		assert.NotEmpty(t, resp.Data)
	})

	t.Run("default limit", func(t *testing.T) {
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
	})
}

// ---------------------------------------------------------------------------
// OpsAgentProcesses
// ---------------------------------------------------------------------------

func TestOpsAgentProcesses_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})

	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{AgentID: "missing"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentProcesses_Success(t *testing.T) {
	processesResp := &opsv1.ListProcessesResponse{
		Processes: []*opsv1.ManagedProcess{
			{
				Name:         "game-server",
				Command:      "/bin/game",
				WorkingDir:   "/opt/game",
				State:        opsv1.ProcessState_PROCESS_STATE_RUNNING,
				Pid:          1234,
				RestartCount: 2,
				LastStart:    timestamppb.Now(),
			},
			{
				Name:    "worker",
				Command: "/bin/worker",
				State:   opsv1.ProcessState_PROCESS_STATE_STOPPED,
			},
		},
	}
	data, _ := proto.Marshal(processesResp)

	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{
			protocol.MsgListProcessesRequest: data,
		}},
	})

	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, "game-server", resp.Data[0].Name)
	assert.Equal(t, "PROCESS_STATE_RUNNING", resp.Data[0].State)
	assert.Equal(t, int32(1234), resp.Data[0].Pid)
	assert.Equal(t, "worker", resp.Data[1].Name)
	assert.Equal(t, "PROCESS_STATE_STOPPED", resp.Data[1].State)
}

// ---------------------------------------------------------------------------
// OpsAgentSystemInfo (beyond cache tests)
// ---------------------------------------------------------------------------

func TestOpsAgentSystemInfo_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})

	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}
	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "missing"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentSystemInfo_FromAgent(t *testing.T) {
	infoResp := &opsv1.SystemInfo{
		Hostname:     "test-host",
		Os:           "linux",
		Arch:         "amd64",
		CpuCores:     8,
		TotalMemory:  16 * 1024 * 1024 * 1024,
		BootTime:     timestamppb.Now(),
		AgentVersion: "v2.0.0",
	}
	data, _ := proto.Marshal(infoResp)

	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{
			protocol.MsgGetSystemInfoRequest: data,
		}},
	})

	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}
	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "test-host", resp.Data.Hostname)
	assert.Equal(t, "linux", resp.Data.OS)
	assert.Equal(t, int32(8), resp.Data.CPUCores)
	assert.Equal(t, "v2.0.0", resp.Data.AgentVersion)

	// Second call should hit cache
	resp2, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, resp2.Code)
	assert.Equal(t, "test-host", resp2.Data.Hostname)
}

// ---------------------------------------------------------------------------
// OpsClientWrapper methods
// ---------------------------------------------------------------------------

func TestOpsClientWrapper_GetSystemInfo(t *testing.T) {
	infoResp := &opsv1.SystemInfo{Hostname: "h", Os: "linux"}
	data, _ := proto.Marshal(infoResp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgGetSystemInfoRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	info, err := wrapper.GetSystemInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "h", info.Hostname)
}

func TestOpsClientWrapper_ListProcesses(t *testing.T) {
	resp := &opsv1.ListProcessesResponse{Processes: []*opsv1.ManagedProcess{{Name: "p1"}}}
	data, _ := proto.Marshal(resp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgListProcessesRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	result, err := wrapper.ListProcesses(context.Background())
	require.NoError(t, err)
	assert.Len(t, result.Processes, 1)
}

func TestOpsClientWrapper_ReportMetrics(t *testing.T) {
	caller := &opsTestCaller{responses: map[uint32][]byte{}}
	wrapper := &OpsClientWrapper{caller: caller}
	report := &opsv1.MetricsReport{Cpu: &opsv1.CpuMetrics{Cores: 4}}
	result, err := wrapper.ReportMetrics(context.Background(), report)
	require.NoError(t, err)
	assert.Equal(t, int32(4), result.Cpu.Cores)
}

func TestOpsClientWrapper_RestartProcess(t *testing.T) {
	resp := &opsv1.RestartProcessResponse{Success: true}
	data, _ := proto.Marshal(resp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgRestartProcessRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	result, err := wrapper.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "p1"})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestOpsClientWrapper_StopProcess(t *testing.T) {
	resp := &opsv1.StopProcessResponse{Success: true}
	data, _ := proto.Marshal(resp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgStopProcessRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	result, err := wrapper.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "p1"})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestOpsClientWrapper_StartProcess(t *testing.T) {
	resp := &opsv1.StartProcessResponse{Pid: 1234}
	data, _ := proto.Marshal(resp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgStartProcessRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	result, err := wrapper.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "p1"})
	require.NoError(t, err)
	assert.Equal(t, int32(1234), result.Pid)
}

func TestOpsClientWrapper_ExecuteCommand(t *testing.T) {
	resp := &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "ok"}
	data, _ := proto.Marshal(resp)
	caller := &opsTestCaller{responses: map[uint32][]byte{protocol.MsgExecuteCommandRequest: data}}
	wrapper := &OpsClientWrapper{caller: caller}
	result, err := wrapper.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{Command: "ls"})
	require.NoError(t, err)
	assert.Equal(t, int32(0), result.ExitCode)
	assert.Equal(t, "ok", result.StdOut)
}

// ---------------------------------------------------------------------------
// Process management logic
// ---------------------------------------------------------------------------

func TestOpsAgentExecCommand_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{AgentID: "missing", Command: "ls"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentExecCommand_Success(t *testing.T) {
	resp := &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "file.txt"}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgExecuteCommandRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, int32(0), result.Data.ExitCode)
	assert.Equal(t, "file.txt", result.Data.StdOut)
}

func TestOpsAgentProcessStart_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{AgentID: "missing", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentProcessStart_Success(t *testing.T) {
	resp := &opsv1.StartProcessResponse{Success: true, Pid: 999}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgStartProcessRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, int32(999), result.Data)
}

func TestOpsAgentProcessStop_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{AgentID: "missing", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentProcessStop_Success(t *testing.T) {
	resp := &opsv1.StopProcessResponse{Success: true}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgStopProcessRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
}

func TestOpsAgentProcessRestart_AgentNotFound(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{agents: map[string]bool{}})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{AgentID: "missing", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
}

func TestOpsAgentProcessRestart_Success(t *testing.T) {
	resp := &opsv1.RestartProcessResponse{Success: true}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgRestartProcessRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type opsTestResolver struct {
	agents map[string]bool
	caller transport.SessionCaller
}

func (r *opsTestResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	if r.agents[agentID] {
		if r.caller != nil {
			return r.caller, true
		}
		return &opsTestCaller{}, true
	}
	return nil, false
}

type opsTestCaller struct {
	responses map[uint32][]byte
}

func (c *opsTestCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	if resp, ok := c.responses[msgID]; ok {
		return msgID, resp, nil
	}
	return msgID, []byte{}, nil
}
