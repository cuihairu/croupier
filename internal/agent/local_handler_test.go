package agent

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// --- Mock Implementations ---

type mockProviderManager struct {
	isPlatformFunc bool
	callResp       []byte
	callErr        error
	callCalled     int
}

func (m *mockProviderManager) IsPlatformFunction(functionID string) bool {
	return m.isPlatformFunc
}

func (m *mockProviderManager) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	m.callCalled++
	return m.callResp, m.callErr
}

type mockOpsServer struct {
	systemInfoResp   *opsv1.SystemInfo
	systemInfoErr    error
	processesResp    *opsv1.ListProcessesResponse
	processesErr     error
	metricsResp      *emptypb.Empty
	metricsErr       error
	restartResp      *opsv1.RestartProcessResponse
	restartErr       error
	stopResp         *opsv1.StopProcessResponse
	stopErr          error
	startResp        *opsv1.StartProcessResponse
	startErr         error
	executeResp      *opsv1.ExecuteCommandResponse
	executeErr       error
	servicesJSONResp []byte
	servicesJSONErr  error
	statusJSONResp   []byte
	statusJSONErr    error
	cronJobsResp     []byte
	cronJobsErr      error
}

func (m *mockOpsServer) GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error) {
	return m.systemInfoResp, m.systemInfoErr
}

func (m *mockOpsServer) ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	return m.processesResp, m.processesErr
}

func (m *mockOpsServer) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	return m.metricsResp, m.metricsErr
}

func (m *mockOpsServer) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	return m.restartResp, m.restartErr
}

func (m *mockOpsServer) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	return m.stopResp, m.stopErr
}

func (m *mockOpsServer) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	return m.startResp, m.startErr
}

func (m *mockOpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	return m.executeResp, m.executeErr
}

func (m *mockOpsServer) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return m.servicesJSONResp, m.servicesJSONErr
}

func (m *mockOpsServer) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	return m.statusJSONResp, m.statusJSONErr
}

func (m *mockOpsServer) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	return m.cronJobsResp, m.cronJobsErr
}

type mockTaskEventReporter struct {
	reportErr    error
	reportCalled int
}

func (m *mockTaskEventReporter) ReportTaskEvent(ctx context.Context, event *sdkv1.TaskEvent) error {
	m.reportCalled++
	return m.reportErr
}

// --- Tests for NewLocalHandler ---

func TestNewLocalHandler(t *testing.T) {
	t.Run("with valid params", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		assert.NotNil(t, handler)
		assert.Equal(t, store, handler.store)
		assert.Equal(t, "/tmp", handler.configDir)
		assert.Equal(t, "agent-1", handler.agentID)
		assert.NotNil(t, handler.tasks)
		assert.NotNil(t, handler.logger)
	})

	t.Run("with nil logger", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		assert.NotNil(t, handler.logger)
	})

	t.Run("with custom logger", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		logger := slog.Default()
		handler := NewLocalHandler(store, "/tmp", "agent-1", logger)

		assert.Equal(t, logger, handler.logger)
	})
}

// --- Tests for SetProviderManager ---

func TestLocalHandler_SetProviderManager(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	pm := &mockProviderManager{}
	handler.SetProviderManager(pm)

	handler.mu.RLock()
	assert.Equal(t, pm, handler.pm)
	handler.mu.RUnlock()
}

// --- Tests for SetOpsServer ---

func TestLocalHandler_SetOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{}
	handler.SetOpsServer(ops)

	handler.mu.RLock()
	assert.Equal(t, ops, handler.opsServer)
	handler.mu.RUnlock()
}

// --- Tests for SetTaskEventReporter ---

func TestLocalHandler_SetTaskEventReporter(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	reporter := &mockTaskEventReporter{}
	handler.SetTaskEventReporter(reporter)

	handler.mu.RLock()
	assert.Equal(t, reporter, handler.reporter)
	handler.mu.RUnlock()
}

// --- Tests for SetTLSConfig ---

func TestLocalHandler_SetTLSConfig(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	cfg := &tlsutil.ClientTLSConfig{}
	handler.SetTLSConfig(cfg)

	handler.mu.RLock()
	assert.Equal(t, cfg, handler.tlsCfg)
	handler.mu.RUnlock()
}

// --- Tests for Handle ---

func TestLocalHandler_Handle(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	t.Run("unknown message type", func(t *testing.T) {
		_, err := handler.Handle(context.Background(), 0xFFFFFF, 0, []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown message type")
	})
}

// --- Tests for TaskRunner ---

func TestTaskRunner(t *testing.T) {
	t.Run("new task runner", func(t *testing.T) {
		r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
			return []byte("null"), nil
		}, nil, nil)
		assert.NotNil(t, r)
		assert.Equal(t, 0, r.Count())
	})

	t.Run("Start tracks task and Cancel reports event", func(t *testing.T) {
		reporter := &mockTaskEventReporter{}
		executed := make(chan struct{})
		r := NewTaskRunner(func(ctx context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
			<-ctx.Done() // block until cancelled
			return nil, ctx.Err()
		}, reporter, nil)

		id := r.Start(&sdkv1.InvokeRequest{FunctionId: "f", Metadata: map[string]string{"task_id": "task-1"}})
		assert.Equal(t, "task-1", id)
		assert.Equal(t, 1, r.Count())

		ok := r.Cancel("task-1")
		assert.True(t, ok)
		_ = executed
	})

	t.Run("Cancel unknown task returns false", func(t *testing.T) {
		r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
			return []byte("null"), nil
		}, nil, nil)
		assert.False(t, r.Cancel("missing"))
	})

	t.Run("Start without metadata generates local task id", func(t *testing.T) {
		reporter := &mockTaskEventReporter{}
		r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
			return []byte("ok"), nil
		}, reporter, nil)

		id := r.Start(&sdkv1.InvokeRequest{FunctionId: "f"})
		assert.Contains(t, id, "task-")
	})
}

// --- Tests for pickInstance ---

func TestLocalHandler_PickInstance(t *testing.T) {
	t.Run("with nil store", func(t *testing.T) {
		handler := &LocalHandler{
			store:  nil,
			logger: slog.Default(),
		}

		_, err := handler.pickInstance("func-1", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance store not initialized")
	})

	t.Run("with empty function ID", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := &LocalHandler{
			store:  store,
			logger: slog.Default(),
		}

		_, err := handler.pickInstance("", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "function ID is required")
	})

	t.Run("with no instances", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := &LocalHandler{
			store:  store,
			logger: slog.Default(),
		}

		_, err := handler.pickInstance("func-1", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not registered")
	})
}

// --- Tests for invokePlatform ---

func TestLocalHandler_InvokePlatform(t *testing.T) {
	t.Run("with nil provider manager", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.invokePlatform(context.Background(), "func-1", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider manager not configured")
	})
}

// --- Tests for handleGetSystemInfo ---

func TestLocalHandler_HandleGetSystemInfo(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleGetSystemInfo(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleGetSystemInfo(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal Empty")
	})
}

// --- Tests for handleListProcesses ---

func TestLocalHandler_HandleListProcesses(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleListProcesses(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleListProcesses(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal Empty")
	})
}

// --- Tests for handleReportMetrics ---

func TestLocalHandler_HandleReportMetrics(t *testing.T) {
	t.Run("with nil ops server - should ack", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		resp, err := handler.handleReportMetrics(context.Background(), []byte{})
		// Should return empty response even with nil ops server
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleReportMetrics(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal MetricsReport")
	})
}

// --- Tests for handleRestartProcess ---

func TestLocalHandler_HandleRestartProcess(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleRestartProcess(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleRestartProcess(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal RestartProcessRequest")
	})
}

// --- Tests for handleStopProcess ---

func TestLocalHandler_HandleStopProcess(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleStopProcess(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})
}

// --- Tests for handleStartProcess ---

func TestLocalHandler_HandleStartProcess(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleStartProcess(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})
}

// --- Tests for handleExecuteCommand ---

func TestLocalHandler_HandleExecuteCommand(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleExecuteCommand(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})
}

// --- Tests for handleListServices ---

func TestLocalHandler_HandleListServices(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleListServices(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})
}

// --- Tests for handleGetServiceStatus ---

func TestLocalHandler_HandleGetServiceStatus(t *testing.T) {
	t.Run("with nil ops server", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleGetServiceStatus(context.Background(), []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ops server not configured")
	})
}

// --- Tests for handleRegisterCapabilities ---

func TestLocalHandler_HandleRegisterCapabilities(t *testing.T) {
	t.Run("with valid empty request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		resp, err := handler.handleRegisterCapabilities(context.Background(), []byte{})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleRegisterCapabilities(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal RegisterCapabilitiesRequest")
	})
}

// --- Tests for hostFromAddr ---

func TestHostFromAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{"empty address", "", ""},
		{"host:port", "localhost:8080", "localhost"},
		{"ip:port", "192.168.1.1:8080", "192.168.1.1"},
		{"ipv6:port", "[::1]:8080", "::1"},
		{"host only", "localhost", "localhost"},
		{"whitespace", "  localhost:8080  ", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostFromAddr(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Tests for fnvIndex ---

func TestFnvIndex(t *testing.T) {
	t.Run("with mod 0", func(t *testing.T) {
		result := fnvIndex("test", 0)
		assert.Equal(t, 0, result)
	})

	t.Run("with mod 1", func(t *testing.T) {
		result := fnvIndex("test", 1)
		assert.Equal(t, 0, result)
	})

	t.Run("with valid mod", func(t *testing.T) {
		result := fnvIndex("test", 10)
		assert.True(t, result >= 0 && result < 10)
	})

	t.Run("deterministic", func(t *testing.T) {
		result1 := fnvIndex("test", 10)
		result2 := fnvIndex("test", 10)
		assert.Equal(t, result1, result2)
	})

	t.Run("different keys different index", func(t *testing.T) {
		// With high probability, different keys should have different indices
		result1 := fnvIndex("key1", 100)
		result2 := fnvIndex("key2", 100)
		// Not guaranteed but very likely
		assert.True(t, result1 >= 0 && result1 < 100)
		assert.True(t, result2 >= 0 && result2 < 100)
	})
}

// --- Tests for mustMarshal ---

func TestMustMarshal(t *testing.T) {
	t.Run("marshal proto message", func(t *testing.T) {
		msg := &agentv1.RegisterResponse{
			SessionId: "test-session",
		}

		result := mustMarshal(msg)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
	})

	t.Run("marshal nil", func(t *testing.T) {
		result := mustMarshal(nil)
		assert.Nil(t, result)
	})
}

// --- Tests for handleStartTask ---

func TestLocalHandler_HandleStartTask(t *testing.T) {
	t.Run("with valid request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		// Create a minimal valid request
		req := &sdkv1.InvokeRequest{
			FunctionId: "test.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleStartTask(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleStartTask(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal InvokeRequest for StartTask")
	})
}

// --- Tests for handleCancelTask ---

func TestLocalHandler_HandleCancelTask(t *testing.T) {
	t.Run("with valid request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.CancelTaskRequest{
			TaskId: "task-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleCancelTask(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleCancelTask(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal CancelTaskRequest")
	})
}

// --- Tests for task event reporting via TaskRunner ---

func TestLocalHandler_TaskEventReporting(t *testing.T) {
	t.Run("task events route through reporter", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		reporter := &mockTaskEventReporter{}
		handler.SetTaskEventReporter(reporter)

		// A StartTask with no provider registered will fail fast; the TaskRunner
		// still emits started + failed events through the reporter.
		req := &sdkv1.InvokeRequest{
			FunctionId: "fn.echo",
			Metadata:   map[string]string{"task_id": "task-evt"},
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleStartTask(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Allow the async task to run and report events.
		time.Sleep(50 * time.Millisecond)
		assert.GreaterOrEqual(t, reporter.reportCalled, 1)
	})

	t.Run("nil reporter does not panic", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)
		// No reporter set; starting a task must not panic and events are dropped.
		req := &sdkv1.InvokeRequest{
			FunctionId: "fn.echo",
			Metadata:   map[string]string{"task_id": "task-nil"},
		}
		data, _ := proto.Marshal(req)
		_, err := handler.handleStartTask(context.Background(), data)
		assert.NoError(t, err)
		time.Sleep(30 * time.Millisecond)
	})
}

// --- Tests for handleProviderConnect ---

func TestLocalHandler_HandleProviderConnect(t *testing.T) {
	t.Run("with valid request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.ProviderConnectRequest{
			ServiceId:   "svc-1",
			Version:     "1.0.0",
			SdkLanguage: "go",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleProviderConnect(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with empty service ID", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.ProviderConnectRequest{
			ServiceId: "",
		}
		data, _ := proto.Marshal(req)

		_, err := handler.handleProviderConnect(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service_id is required")
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleProviderConnect(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal ProviderConnectRequest")
	})
}

// --- Tests for handleProviderHeartbeat ---

func TestLocalHandler_HandleProviderHeartbeat(t *testing.T) {
	t.Run("with valid request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.ProviderHeartbeatRequest{
			ServiceId: "svc-1",
			SessionId: "session-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleProviderHeartbeat(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleProviderHeartbeat(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal ProviderHeartbeatRequest")
	})
}

// --- Tests for handleProviderDrain ---

func TestLocalHandler_HandleProviderDrain(t *testing.T) {
	t.Run("with valid request", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.ProviderDrainRequest{
			SessionId: "session-1",
			Reason:    "deploy",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleProviderDrain(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("with invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleProviderDrain(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal ProviderDrainRequest")
	})
}

// --- Helper to create LocalStore ---

func newTestLocalStore() *agentlocal.LocalStore {
	return agentlocal.NewLocalStore()
}

// --- Tests for handleRequest dispatch ---

func TestLocalHandler_HandleRequest(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	t.Run("invoke request dispatches to handleInvoke", func(t *testing.T) {
		req := &sdkv1.InvokeRequest{
			FunctionId: "test.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		// Will fail because no instance registered, but dispatches correctly
		_, err := handler.handleRequest(context.Background(), protocol.MsgInvokeRequest, data)
		assert.Error(t, err) // Expected: function not registered
	})

	t.Run("start task request dispatches to handleStartTask", func(t *testing.T) {
		req := &sdkv1.InvokeRequest{
			FunctionId: "test.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleRequest(context.Background(), protocol.MsgStartTaskRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("cancel task request dispatches to handleCancelTask", func(t *testing.T) {
		req := &sdkv1.CancelTaskRequest{
			TaskId: "task-1",
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleRequest(context.Background(), protocol.MsgCancelTaskRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("get system info request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&emptypb.Empty{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgGetSystemInfoRequest, data)
		assert.Error(t, err) // ops server not configured
	})

	t.Run("list processes request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&emptypb.Empty{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgListProcessesRequest, data)
		assert.Error(t, err)
	})

	t.Run("report metrics request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&opsv1.MetricsReport{})
		resp, err := handler.handleRequest(context.Background(), protocol.MsgReportMetricsRequest, data)
		assert.NoError(t, err) // acks even without ops
		assert.NotNil(t, resp)
	})

	t.Run("restart process request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&opsv1.RestartProcessRequest{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgRestartProcessRequest, data)
		assert.Error(t, err)
	})

	t.Run("stop process request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&opsv1.StopProcessRequest{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgStopProcessRequest, data)
		assert.Error(t, err)
	})

	t.Run("start process request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&opsv1.StartProcessRequest{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgStartProcessRequest, data)
		assert.Error(t, err)
	})

	t.Run("execute command request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&opsv1.ExecuteCommandRequest{})
		_, err := handler.handleRequest(context.Background(), protocol.MsgExecuteCommandRequest, data)
		assert.Error(t, err)
	})

	t.Run("list services request dispatches", func(t *testing.T) {
		_, err := handler.handleRequest(context.Background(), protocol.MsgListServicesRequest, []byte("{}"))
		assert.Error(t, err)
	})

	t.Run("get service status request dispatches", func(t *testing.T) {
		_, err := handler.handleRequest(context.Background(), protocol.MsgGetServiceStatusRequest, []byte("{}"))
		assert.Error(t, err)
	})

	t.Run("register capabilities request dispatches", func(t *testing.T) {
		data, _ := proto.Marshal(&agentv1.RegisterCapabilitiesRequest{})
		resp, err := handler.handleRequest(context.Background(), protocol.MsgRegisterCapabilitiesReq, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("provider connect request dispatches", func(t *testing.T) {
		req := &sdkv1.ProviderConnectRequest{
			ServiceId: "svc-1",
		}
		data, _ := proto.Marshal(req)
		resp, err := handler.handleRequest(context.Background(), protocol.MsgProviderConnectRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("provider heartbeat request dispatches", func(t *testing.T) {
		req := &sdkv1.ProviderHeartbeatRequest{
			SessionId: "session-1",
		}
		data, _ := proto.Marshal(req)
		resp, err := handler.handleRequest(context.Background(), protocol.MsgProviderHeartbeatRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("provider drain request dispatches", func(t *testing.T) {
		req := &sdkv1.ProviderDrainRequest{
			SessionId: "session-1",
		}
		data, _ := proto.Marshal(req)
		resp, err := handler.handleRequest(context.Background(), protocol.MsgProviderDrainRequest, data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("unknown message type", func(t *testing.T) {
		_, err := handler.handleRequest(context.Background(), 0xFFFFFF, []byte{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown message type")
	})
}

// --- Tests for handleInvoke ---

func TestLocalHandler_HandleInvoke(t *testing.T) {
	t.Run("with platform function", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		pm := &mockProviderManager{
			isPlatformFunc: true,
			callResp:       []byte(`{"result":"ok"}`),
		}
		handler.SetProviderManager(pm)

		req := &sdkv1.InvokeRequest{
			FunctionId: "platform.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		resp, err := handler.handleInvoke(context.Background(), data)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 1, pm.callCalled)
	})

	t.Run("with platform function error", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		pm := &mockProviderManager{
			isPlatformFunc: true,
			callErr:        fmt.Errorf("call failed"),
		}
		handler.SetProviderManager(pm)

		req := &sdkv1.InvokeRequest{
			FunctionId: "platform.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		_, err := handler.handleInvoke(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider call failed")
	})

	t.Run("with regular function and no instances", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		req := &sdkv1.InvokeRequest{
			FunctionId: "game.func",
			Payload:    []byte("{}"),
		}
		data, _ := proto.Marshal(req)

		_, err := handler.handleInvoke(context.Background(), data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not registered")
	})

	t.Run("invalid protobuf data", func(t *testing.T) {
		store := agentlocal.NewLocalStore()
		handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

		_, err := handler.handleInvoke(context.Background(), []byte("invalid"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal InvokeRequest")
	})
}

// --- Tests for invokePlatform success ---

func TestLocalHandler_InvokePlatform_Success(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	pm := &mockProviderManager{
		isPlatformFunc: true,
		callResp:       []byte(`{"data":"test"}`),
	}
	handler.SetProviderManager(pm)

	req := &sdkv1.InvokeRequest{
		FunctionId: "platform.func",
		Payload:    []byte(`{"key":"value"}`),
	}

	resp, err := handler.invokePlatform(context.Background(), "platform.func", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response is a valid InvokeResponse
	invokeResp := &sdkv1.InvokeResponse{}
	err = proto.Unmarshal(resp, invokeResp)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`{"data":"test"}`), invokeResp.Payload)
}

// --- Tests for pickInstance with registered instances ---

func TestLocalHandler_PickInstance_WithInstances(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := &LocalHandler{
		store:  store,
		logger: slog.Default(),
	}

	// Register an instance
	store.Register("provider-1", "service-1", "localhost:8080", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "game.player.get", Version: "1.0.0"},
	}, nil)

	addr, err := handler.pickInstance("game.player.get", nil)
	assert.NoError(t, err)
	assert.Equal(t, "localhost:8080", addr)
}

// --- Tests for handleGetSystemInfo with ops server ---

func TestLocalHandler_HandleGetSystemInfo_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		systemInfoResp: &opsv1.SystemInfo{
			Hostname: "test-host",
		},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&emptypb.Empty{})
	resp, err := handler.handleGetSystemInfo(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify response
	info := &opsv1.SystemInfo{}
	err = proto.Unmarshal(resp, info)
	assert.NoError(t, err)
	assert.Equal(t, "test-host", info.Hostname)
}

// --- Tests for handleListProcesses with ops server ---

func TestLocalHandler_HandleListProcesses_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		processesResp: &opsv1.ListProcessesResponse{
			Processes: []*opsv1.ManagedProcess{
				{Pid: 1234, Name: "test-process"},
			},
		},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&emptypb.Empty{})
	resp, err := handler.handleListProcesses(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleReportMetrics with ops server ---

func TestLocalHandler_HandleReportMetrics_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		metricsResp: &emptypb.Empty{},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.MetricsReport{})
	resp, err := handler.handleReportMetrics(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleRestartProcess with ops server ---

func TestLocalHandler_HandleRestartProcess_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		restartResp: &opsv1.RestartProcessResponse{Success: true},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.RestartProcessRequest{ProcessName: "test-process"})
	resp, err := handler.handleRestartProcess(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleStopProcess with ops server ---

func TestLocalHandler_HandleStopProcess_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		stopResp: &opsv1.StopProcessResponse{Success: true},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.StopProcessRequest{ProcessName: "test-process"})
	resp, err := handler.handleStopProcess(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleStartProcess with ops server ---

func TestLocalHandler_HandleStartProcess_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		startResp: &opsv1.StartProcessResponse{Pid: 5678},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.StartProcessRequest{ProcessName: "test-process"})
	resp, err := handler.handleStartProcess(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleExecuteCommand with ops server ---

func TestLocalHandler_HandleExecuteCommand_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		executeResp: &opsv1.ExecuteCommandResponse{
			StdOut:   "command output",
			ExitCode: 0,
		},
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.ExecuteCommandRequest{Command: "ls"})
	resp, err := handler.handleExecuteCommand(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleListServices with ops server ---

func TestLocalHandler_HandleListServices_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		servicesJSONResp: []byte(`{"services":["svc-1"]}`),
	}
	handler.SetOpsServer(ops)

	resp, err := handler.handleListServices(context.Background(), []byte("{}"))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleGetServiceStatus with ops server ---

func TestLocalHandler_HandleGetServiceStatus_WithOps(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		statusJSONResp: []byte(`{"status":"running"}`),
	}
	handler.SetOpsServer(ops)

	resp, err := handler.handleGetServiceStatus(context.Background(), []byte("{}"))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for handleCancelTask with existing task ---

func TestLocalHandler_HandleCancelTask_WithExistingTask(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	reporter := &mockTaskEventReporter{}
	handler.SetTaskEventReporter(reporter)

	// Register a task through the runner so it is tracked for cancellation.
	// The executor blocks until cancelled, mirroring a long-running task.
	handler.tasks = NewTaskRunner(func(ctx context.Context, _ *sdkv1.InvokeRequest) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, reporter, nil)
	handler.tasks.Start(&sdkv1.InvokeRequest{
		FunctionId: "fn.block",
		Metadata:   map[string]string{"task_id": "task-1"},
	})

	req := &sdkv1.CancelTaskRequest{
		TaskId: "task-1",
	}
	data, _ := proto.Marshal(req)

	resp, err := handler.handleCancelTask(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// cancel_requested event is emitted by the runner on Cancel.
	assert.GreaterOrEqual(t, reporter.reportCalled, 1)
}

// --- Tests for task reporting with nil event safety ---

func TestLocalHandler_TaskRunner_NilReporterSafe(t *testing.T) {
	// A TaskRunner with a nil reporter must execute tasks and drop events
	// without panicking — mirrors the handler before a reporter is wired.
	r := NewTaskRunner(func(context.Context, *sdkv1.InvokeRequest) ([]byte, error) {
		return []byte("null"), nil
	}, nil, nil)
	r.Start(&sdkv1.InvokeRequest{FunctionId: "f", Metadata: map[string]string{"task_id": "t"}})
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 0, r.Count()) // task completes and untracks itself
}

// --- Tests for handleProviderConnect with functions ---

func TestLocalHandler_HandleProviderConnect_WithFunctions(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	req := &sdkv1.ProviderConnectRequest{
		ServiceId: "svc-1",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{
			{Id: "func-1", Version: "1.0.0"},
			{Id: "func-2", Version: "2.0.0"},
		},
	}
	data, _ := proto.Marshal(req)

	resp, err := handler.handleProviderConnect(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify functions were registered
	snap := store.List()
	assert.NotEmpty(t, snap["func-1"])
	assert.NotEmpty(t, snap["func-2"])
}

func TestLocalHandler_HandleProviderConnect_PreservesDescriptorMetadata(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	req := &sdkv1.ProviderConnectRequest{
		ServiceId: "svc-1",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{
			{
				Id:          "game.player.ban",
				Version:     "1.2.3",
				Summary:     "Ban player",
				Description: "Ban a player account",
				Resource:    "player",
				Risk:        "danger",
				Operation:   "ban",
				Permission:  "player.ban",
			},
		},
	}
	data, _ := proto.Marshal(req)

	resp, err := handler.handleProviderConnect(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	meta := store.FunctionMetadata()["game.player.ban"]
	if assert.NotNil(t, meta) {
		assert.Equal(t, "Ban player", meta.Summary)
		assert.Equal(t, "Ban a player account", meta.Description)
		assert.Equal(t, "player", meta.Resource)
		assert.Equal(t, "danger", meta.Risk)
		assert.Equal(t, "ban", meta.Operation)
		assert.Equal(t, "player.ban", meta.Permission)
		assert.Contains(t, meta.OpenAPIOperation, `"x-resource":"player"`)
		assert.Contains(t, meta.OpenAPIOperation, `"x-risk":"danger"`)
		assert.Contains(t, meta.OpenAPIOperation, `"x-operation":"ban"`)
		assert.Contains(t, meta.OpenAPIOperation, `"x-permission":"player.ban"`)
	}
}

// --- Tests for handleProviderHeartbeat with empty session ID ---

func TestLocalHandler_HandleProviderHeartbeat_EmptySessionID(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	req := &sdkv1.ProviderHeartbeatRequest{
		SessionId: "",
	}
	data, _ := proto.Marshal(req)

	_, err := handler.handleProviderHeartbeat(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session_id is required")
}

// --- Tests for handleProviderDrain with empty session ID ---

func TestLocalHandler_HandleProviderDrain_EmptySessionID(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	req := &sdkv1.ProviderDrainRequest{
		SessionId: "",
	}
	data, _ := proto.Marshal(req)

	_, err := handler.handleProviderDrain(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session_id is required")
}

// --- Tests for ops handler error paths ---

func TestLocalHandler_HandleGetSystemInfo_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		systemInfoErr: fmt.Errorf("system info error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&emptypb.Empty{})
	_, err := handler.handleGetSystemInfo(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get system info")
}

func TestLocalHandler_HandleListProcesses_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		processesErr: fmt.Errorf("list processes error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&emptypb.Empty{})
	_, err := handler.handleListProcesses(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list processes")
}

func TestLocalHandler_HandleReportMetrics_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		metricsErr: fmt.Errorf("metrics error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.MetricsReport{})
	_, err := handler.handleReportMetrics(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "report metrics")
}

func TestLocalHandler_HandleRestartProcess_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		restartErr: fmt.Errorf("restart error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.RestartProcessRequest{ProcessName: "test"})
	_, err := handler.handleRestartProcess(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restart process")
}

func TestLocalHandler_HandleStopProcess_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		stopErr: fmt.Errorf("stop error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.StopProcessRequest{ProcessName: "test"})
	_, err := handler.handleStopProcess(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stop process")
}

func TestLocalHandler_HandleStartProcess_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		startErr: fmt.Errorf("start error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.StartProcessRequest{ProcessName: "test"})
	_, err := handler.handleStartProcess(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start process")
}

func TestLocalHandler_HandleExecuteCommand_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		executeErr: fmt.Errorf("execute error"),
	}
	handler.SetOpsServer(ops)

	data, _ := proto.Marshal(&opsv1.ExecuteCommandRequest{Command: "test"})
	_, err := handler.handleExecuteCommand(context.Background(), data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execute command")
}

func TestLocalHandler_HandleListServices_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		servicesJSONErr: fmt.Errorf("services error"),
	}
	handler.SetOpsServer(ops)

	_, err := handler.handleListServices(context.Background(), []byte("{}"))
	assert.Error(t, err)
}

func TestLocalHandler_HandleGetServiceStatus_Error(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	ops := &mockOpsServer{
		statusJSONErr: fmt.Errorf("status error"),
	}
	handler.SetOpsServer(ops)

	_, err := handler.handleGetServiceStatus(context.Background(), []byte("{}"))
	assert.Error(t, err)
}

// --- Tests for handleProviderHeartbeat with registered provider ---

func TestLocalHandler_HandleProviderHeartbeat_WithRegisteredProvider(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := NewLocalHandler(store, "/tmp", "agent-1", nil)

	// Register a provider first
	store.Register("session-1", "service-1", "localhost:8080", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "func-1", Version: "1.0.0"},
	}, nil)

	req := &sdkv1.ProviderHeartbeatRequest{
		SessionId: "session-1",
	}
	data, _ := proto.Marshal(req)

	resp, err := handler.handleProviderHeartbeat(context.Background(), data)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Tests for mustMarshal edge cases ---

func TestMustMarshal_EdgeCases(t *testing.T) {
	t.Run("empty proto message", func(t *testing.T) {
		msg := &sdkv1.InvokeRequest{}
		result := mustMarshal(msg)
		assert.NotNil(t, result)
	})

	t.Run("nil payload in invoke response", func(t *testing.T) {
		msg := &sdkv1.InvokeResponse{
			Payload: nil,
		}
		result := mustMarshal(msg)
		assert.NotNil(t, result)
	})
}

// --- Tests for pickInstance with metadata ---

func TestLocalHandler_PickInstance_WithMetadata(t *testing.T) {
	store := agentlocal.NewLocalStore()
	handler := &LocalHandler{
		store:  store,
		logger: slog.Default(),
	}

	// Register an instance
	store.Register("provider-1", "service-1", "localhost:8080", "1.0.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "game.player.get", Version: "1.0.0"},
	}, nil)

	metadata := map[string]string{"key": "value"}
	addr, err := handler.pickInstance("game.player.get", metadata)
	assert.NoError(t, err)
	assert.Equal(t, "localhost:8080", addr)
}
