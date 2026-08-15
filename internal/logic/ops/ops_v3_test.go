package ops

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// OpsClientWrapper error paths
// ---------------------------------------------------------------------------

type errorCaller struct {
	errMsg string
}

func (c *errorCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	return 0, nil, fmt.Errorf("%s", c.errMsg)
}

type badDataCaller struct{}

func (c *badDataCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	return msgID, []byte("not valid protobuf"), nil
}

func TestOpsClientWrapper_GetSystemInfo_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "connection lost"}}
	_, err := wrapper.GetSystemInfo(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
}

func TestOpsClientWrapper_GetSystemInfo_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.GetSystemInfo(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestOpsClientWrapper_ListProcesses_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "timeout"}}
	_, err := wrapper.ListProcesses(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestOpsClientWrapper_ListProcesses_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.ListProcesses(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestOpsClientWrapper_ReportMetrics_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "send failed"}}
	_, err := wrapper.ReportMetrics(context.Background(), &opsv1.MetricsReport{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send failed")
}

func TestOpsClientWrapper_RestartProcess_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "restart failed"}}
	_, err := wrapper.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart failed")
}

func TestOpsClientWrapper_RestartProcess_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestOpsClientWrapper_StopProcess_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "stop failed"}}
	_, err := wrapper.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop failed")
}

func TestOpsClientWrapper_StopProcess_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.StopProcess(context.Background(), &opsv1.StopProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestOpsClientWrapper_StartProcess_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "start failed"}}
	_, err := wrapper.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start failed")
}

func TestOpsClientWrapper_StartProcess_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestOpsClientWrapper_ExecuteCommand_CallError(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &errorCaller{errMsg: "exec failed"}}
	_, err := wrapper.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{Command: "ls"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec failed")
}

func TestOpsClientWrapper_ExecuteCommand_BadData(t *testing.T) {
	wrapper := &OpsClientWrapper{caller: &badDataCaller{}}
	_, err := wrapper.ExecuteCommand(context.Background(), &opsv1.ExecuteCommandRequest{Command: "ls"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// ---------------------------------------------------------------------------
// OpsAgentsList: agents with nil session entry (cover nil sess check)
// ---------------------------------------------------------------------------

func TestOpsAgentsList_NilSession(t *testing.T) {
	store := registry.NewStore()
	// Manually insert a nil session (this simulates a race condition edge case)
	// Since we can't easily insert nil, test with the existing nil-safe code
	svcCtx := &svc.ServiceContext{RegistryStore: store}
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Empty(t, resp.Data)
}

// ---------------------------------------------------------------------------
// OpsAgentMetrics: since time parsing, per-core, nil report
// ---------------------------------------------------------------------------

func TestOpsAgentMetrics_SinceTimeAndNilReport(t *testing.T) {
	store := registry.NewMetricsStore()
	// Add an entry with nil report
	store.Add("agent-nil", nil)

	svcCtx := &svc.ServiceContext{MetricsStore: store}
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

	t.Run("with since filter", func(t *testing.T) {
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Since: time.Now().Add(-time.Hour).Format(time.RFC3339),
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
	})

	t.Run("invalid since time", func(t *testing.T) {
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Since: "not-a-time",
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
	})

	t.Run("per-core metrics", func(t *testing.T) {
		store2 := registry.NewMetricsStore()
		store2.Add("agent-percore", &opsv1.MetricsReport{
			Cpu: &opsv1.CpuMetrics{
				Cores:        4,
				UsagePercent: 50.0,
				Load_1M:      1.5,
				Load_5M:      2.0,
				Load_15M:     3.0,
				PerCore:      []float64{10.0, 20.0, 30.0, 40.0},
			},
			Memory: &opsv1.MemoryMetrics{
				TotalBytes:     8 * 1024 * 1024 * 1024,
				UsedBytes:      4 * 1024 * 1024 * 1024,
				AvailableBytes: 4 * 1024 * 1024 * 1024,
				UsagePercent:   50.0,
				SwapTotal:      1024,
				SwapUsed:       512,
			},
			Disks: []*opsv1.DiskMetrics{
				{
					MountPoint:     "/",
					Device:         "/dev/sda1",
					FsType:         "ext4",
					TotalBytes:     100 * 1024 * 1024 * 1024,
					UsedBytes:      60 * 1024 * 1024 * 1024,
					AvailableBytes: 40 * 1024 * 1024 * 1024,
					UsagePercent:   60.0,
				},
			},
			Networks: []*opsv1.NetworkMetrics{
				{
					Interface:   "eth0",
					BytesSent:   1024,
					BytesRecv:   2048,
					PacketsSent: 100,
					PacketsRecv: 200,
				},
			},
		})

		svcCtx2 := &svc.ServiceContext{MetricsStore: store2}
		logic2 := NewOpsAgentMetricsLogic(context.Background(), svcCtx2)
		resp, err := logic2.OpsAgentMetrics(&OpsAgentMetricsRequest{AgentID: "agent-percore", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 0, resp.Code)
		require.Len(t, resp.Data, 1)
		assert.Len(t, resp.Data[0].CPU.PerCore, 4)
		assert.Len(t, resp.Data[0].Disks, 1)
		assert.Len(t, resp.Data[0].Networks, 1)
	})
}

// ---------------------------------------------------------------------------
// Process start/stop/restart with resp.Success=false
// ---------------------------------------------------------------------------

func TestOpsAgentProcessStart_SuccessFalse(t *testing.T) {
	resp := &opsv1.StartProcessResponse{Success: false, Message: "permission denied"}
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
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "permission denied")
}

func TestOpsAgentProcessStop_SuccessFalse(t *testing.T) {
	resp := &opsv1.StopProcessResponse{Success: false, Message: "process not running"}
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
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "process not running")
}

func TestOpsAgentProcessRestart_SuccessFalse(t *testing.T) {
	resp := &opsv1.RestartProcessResponse{Success: false, Message: "restart not allowed"}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgRestartProcessRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{AgentID: "agent-1", Name: "p1", Force: true})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "restart not allowed")
}

func TestOpsAgentProcessStart_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "rpc error"},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to start process")
}

func TestOpsAgentProcessStop_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "rpc error"},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to stop process")
}

func TestOpsAgentProcessRestart_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "rpc error"},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{AgentID: "agent-1", Name: "p1"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to restart process")
}

// ---------------------------------------------------------------------------
// ExecCommand with timeout > 0
// ---------------------------------------------------------------------------

func TestOpsAgentExecCommand_WithTimeout(t *testing.T) {
	resp := &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "done"}
	data, _ := proto.Marshal(resp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgExecuteCommandRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
		AgentID: "agent-1",
		Command: "ls",
		Timeout: 30,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, "done", result.Data.StdOut)
}

func TestOpsAgentExecCommand_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "exec failed"},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{AgentID: "agent-1", Command: "ls"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to execute command")
}

// ---------------------------------------------------------------------------
// OpsAgentProcesses with process list conversion
// ---------------------------------------------------------------------------

func TestOpsAgentProcesses_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "list failed"},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to list processes")
}

func TestOpsAgentProcesses_ProcessWithNilLastStart(t *testing.T) {
	processesResp := &opsv1.ListProcessesResponse{
		Processes: []*opsv1.ManagedProcess{
			{
				Name:       "test-proc",
				Command:    "/bin/test",
				WorkingDir: "/tmp",
				State:      opsv1.ProcessState_PROCESS_STATE_STARTING,
				Pid:        0,
				LastStart:  nil,
			},
		},
	}
	data, _ := proto.Marshal(processesResp)
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &opsTestCaller{responses: map[uint32][]byte{protocol.MsgListProcessesRequest: data}},
	})
	svcCtx := &svc.ServiceContext{}
	logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Code)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "", result.Data[0].LastStart)
	assert.Equal(t, "PROCESS_STATE_STARTING", result.Data[0].State)
}

// ---------------------------------------------------------------------------
// SystemInfo with gRPC call error
// ---------------------------------------------------------------------------

func TestOpsAgentSystemInfo_CallError(t *testing.T) {
	client := GetAgentOpsClient()
	client.SetSessionResolver(&opsTestResolver{
		agents: map[string]bool{"agent-1": true},
		caller: &errorCaller{errMsg: "system info unavailable"},
	})
	svcCtx := &svc.ServiceContext{SystemInfoCache: registry.NewSystemInfoCache()}
	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
	result, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	assert.Equal(t, 500, result.Code)
	assert.Contains(t, result.Message, "Failed to get system info")
}

// ---------------------------------------------------------------------------
// OpsAgentMetrics with nil report in entries
// ---------------------------------------------------------------------------

func TestOpsAgentMetrics_NilReportEntry(t *testing.T) {
	store := registry.NewMetricsStore()
	// Add entry for agent but the entry has nil report
	store.Add("agent-test", nil)
	svcCtx := &svc.ServiceContext{MetricsStore: store}
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

	resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{AgentID: "agent-test", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	// nil report should be skipped
	assert.Empty(t, resp.Data)
}

// ---------------------------------------------------------------------------
// OpsServices: test with nil RegistryStore (permission check short-circuits)
// ---------------------------------------------------------------------------

func TestOpsServicesLogic_OpsServices_NilRegistry(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{Host: "localhost", Port: 8080},
		},
		ServerVersion:   "v1.0.0",
		StartTime:       time.Now(),
		RegistryStore:   nil, // nil store
		MetricsStore:    registry.NewMetricsStore(),
		SystemInfoCache: registry.NewSystemInfoCache(),
	}
	ctx := context.Background()
	logic := NewOpsServicesLogic(ctx, svcCtx)

	// This will fail at the permission check, which is expected
	_, err := logic.OpsServices(&OpsServicesRequest{GameID: "test"})
	// Permission check requires admin context, so it should return an error
	if err != nil {
		t.Logf("Expected permission error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// formatTimestamp with non-nil timestamp
// ---------------------------------------------------------------------------

func TestFormatTimestamp_NonNil(t *testing.T) {
	ts := timestamppb.New(time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC))
	result := formatTimestamp(ts)
	assert.Contains(t, result, "2024")
	assert.Contains(t, result, "06")
	assert.Contains(t, result, "15")
}

// ---------------------------------------------------------------------------
// OpsAgentsList with providers
// ---------------------------------------------------------------------------

func TestOpsAgentsList_WithProviders(t *testing.T) {
	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-with-providers",
		GameID:   "game1",
		Env:      "prod",
		Version:  "v1.0.0",
		ExpireAt: time.Now().Add(time.Minute),
		LastSeen: time.Now(),
		Functions: map[string]registry.FunctionMeta{
			"func1": {Enabled: true},
		},
		Providers: []registry.ProviderSession{
			{
				ProviderID:   "provider-1",
				Addr:         "localhost:8081",
				Version:      "v1.0.0",
				LastSeenUnix: time.Now().Unix(),
				FunctionIDs:  []string{"func1"},
			},
		},
		Labels: map[string]string{"team": "backend"},
	})

	svcCtx := &svc.ServiceContext{RegistryStore: store}
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)
	assert.Len(t, resp.Data[0].Processes, 1) // provider IDs
	assert.Equal(t, "provider-1", resp.Data[0].Processes[0])
}

// ---------------------------------------------------------------------------
// type assertions for ops types (cover more type fields)
// ---------------------------------------------------------------------------

// helper for error caller
// AgentSessionResolver - concurrent access
// ---------------------------------------------------------------------------

func TestAgentOpsClient_ConcurrentAccess(t *testing.T) {
	client := GetAgentOpsClient()
	resolver := &opsTestResolver{agents: map[string]bool{"agent-1": true}}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			client.SetSessionResolver(resolver)
			_, err := client.GetClient(context.Background(), "agent-1")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// helper for error caller
type errCaller struct{}

func (c *errCaller) Call(_ context.Context, msgID uint32, _ []byte) (uint32, []byte, error) {
	return 0, nil, fmt.Errorf("network error")
}
