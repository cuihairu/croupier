package nng

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockProviderManager is a mock implementation of ProviderManager
type mockProviderManager struct {
	isPlatformFunc func(functionID string) bool
	callFunc       func(ctx context.Context, functionID string, request []byte) ([]byte, error)
}

func (m *mockProviderManager) IsPlatformFunction(functionID string) bool {
	if m.isPlatformFunc != nil {
		return m.isPlatformFunc(functionID)
	}
	return false
}

func (m *mockProviderManager) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, functionID, request)
	}
	return nil, errors.New("not implemented")
}

// mockOpsServer is a mock implementation of OpsServerWrapper
type mockOpsServer struct {
	getSystemInfoFunc        func(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error)
	listProcessesFunc        func(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error)
	reportMetricsFunc        func(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error)
	restartProcessFunc       func(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error)
	stopProcessFunc          func(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error)
	startProcessFunc         func(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error)
	executeCommandFunc       func(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error)
	listServicesJSONFunc     func(ctx context.Context, jsonReq []byte) ([]byte, error)
	getServiceStatusJSONFunc func(ctx context.Context, jsonReq []byte) ([]byte, error)
	listCronJobsJSONFunc     func(ctx context.Context) ([]byte, error)
}

func (m *mockOpsServer) GetSystemInfo(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error) {
	if m.getSystemInfoFunc != nil {
		return m.getSystemInfoFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) ListProcesses(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
	if m.listProcessesFunc != nil {
		return m.listProcessesFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) ReportMetrics(ctx context.Context, req *opsv1.MetricsReport) (*emptypb.Empty, error) {
	if m.reportMetricsFunc != nil {
		return m.reportMetricsFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *mockOpsServer) RestartProcess(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
	if m.restartProcessFunc != nil {
		return m.restartProcessFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) StopProcess(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
	if m.stopProcessFunc != nil {
		return m.stopProcessFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) StartProcess(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
	if m.startProcessFunc != nil {
		return m.startProcessFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) ExecuteCommand(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
	if m.executeCommandFunc != nil {
		return m.executeCommandFunc(ctx, req)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) ListServicesJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	if m.listServicesJSONFunc != nil {
		return m.listServicesJSONFunc(ctx, jsonReq)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) GetServiceStatusJSON(ctx context.Context, jsonReq []byte) ([]byte, error) {
	if m.getServiceStatusJSONFunc != nil {
		return m.getServiceStatusJSONFunc(ctx, jsonReq)
	}
	return nil, errors.New("not configured")
}

func (m *mockOpsServer) ListCronJobsJSON(ctx context.Context) ([]byte, error) {
	if m.listCronJobsJSONFunc != nil {
		return m.listCronJobsJSONFunc(ctx)
	}
	return nil, errors.New("not configured")
}

// TestNewAgentServer tests creating a new agent server
func TestNewAgentServer(t *testing.T) {
	tests := []struct {
		name           string
		addr           string
		wantAddrCount  int
		wantPrimarySet bool
	}{
		{
			name:           "Single address",
			addr:           ":19090",
			wantAddrCount:  1,
			wantPrimarySet: true,
		},
		{
			name:           "Multiple addresses with commas",
			addr:           ":19090,ipc://croupier-agent",
			wantAddrCount:  2,
			wantPrimarySet: true,
		},
		{
			name:           "Empty address uses default",
			addr:           "",
			wantAddrCount:  1,
			wantPrimarySet: false,
		},
		{
			name:           "Address with spaces",
			addr:           " :19090 , ipc://test ",
			wantAddrCount:  2,
			wantPrimarySet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := agentlocal.NewLocalStore()
			server := NewAgentServer(tt.addr, store)

			if len(server.addrs) != tt.wantAddrCount {
				t.Errorf("NewAgentServer() addrs count = %d, want %d", len(server.addrs), tt.wantAddrCount)
			}

			if tt.wantPrimarySet && server.addr == "" {
				t.Errorf("NewAgentServer() primary address should be set")
			}

			if server.store == nil {
				t.Errorf("NewAgentServer() store should not be nil")
			}

			if server.jobs == nil {
				t.Errorf("NewAgentServer() jobs should not be nil")
			}

			if server.ctx == nil {
				t.Errorf("NewAgentServer() ctx should not be nil")
			}

			if server.logger == nil {
				t.Errorf("NewAgentServer() logger should not be nil")
			}
		})
	}
}

// TestNewAgentServerWithAddrs tests creating a server with explicit addresses
func TestNewAgentServerWithAddrs(t *testing.T) {
	tests := []struct {
		name          string
		addrs         []ListenAddr
		wantAddrCount int
	}{
		{
			name:          "Single address",
			addrs:         []ListenAddr{ParseListenAddr(":19090")},
			wantAddrCount: 1,
		},
		{
			name:          "Multiple addresses",
			addrs:         []ListenAddr{ParseListenAddr(":19090"), ParseListenAddr("ipc://test")},
			wantAddrCount: 2,
		},
		{
			name:          "Empty addresses uses default",
			addrs:         []ListenAddr{},
			wantAddrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := agentlocal.NewLocalStore()
			server := NewAgentServerWithAddrs(tt.addrs, store)

			if len(server.addrs) != tt.wantAddrCount {
				t.Errorf("NewAgentServerWithAddrs() addrs count = %d, want %d", len(server.addrs), tt.wantAddrCount)
			}
		})
	}
}

// TestAgentServerSetters tests setter methods
func TestAgentServerSetters(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":19090", store)

	// Test SetProviderManager
	pm := &mockProviderManager{}
	server.SetProviderManager(pm)
	if server.providerManager == nil {
		t.Errorf("SetProviderManager() failed")
	}

	// Test SetOpsServer
	ops := &mockOpsServer{}
	server.SetOpsServer(ops)
	if server.opsServer == nil {
		t.Errorf("SetOpsServer() failed")
	}

	// Test SetTLSConfig (can be nil)
	server.SetTLSConfig(nil)
	// Just verify it doesn't panic

	// Test SetLogger
	server.SetLogger(nil)
	// Just verify it doesn't panic
}

// TestAgentServerGetAddr tests GetAddr method
func TestAgentServerGetAddr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single address",
			input:    ":19090",
			expected: ":19090",
		},
		{
			name:     "Multiple addresses",
			input:    ":19090,ipc://test",
			expected: ":19090,ipc://test",
		},
		{
			name:     "Empty address",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := agentlocal.NewLocalStore()
			server := NewAgentServer(tt.input, store)

			if got := server.GetAddr(); got != tt.expected {
				t.Errorf("GetAddr() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestAgentServerGetAddrs tests GetAddrs method
func TestAgentServerGetAddrs(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":19090,ipc://test", store)

	addrs := server.GetAddrs()
	if len(addrs) != 2 {
		t.Errorf("GetAddrs() count = %d, want 2", len(addrs))
	}

	// Verify addresses are correct
	if addrs[0].Transport != "tcp" {
		t.Errorf("GetAddrs()[0].Transport = %q, want tcp", addrs[0].Transport)
	}
	if addrs[1].Transport != "ipc" {
		t.Errorf("GetAddrs()[1].Transport = %q, want ipc", addrs[1].Transport)
	}
}

// TestAgentServerGetLocalAddrs tests GetLocalAddrs method
func TestAgentServerGetLocalAddrs(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":19090,ipc://test", store)

	urls := server.GetLocalAddrs()
	if len(urls) != 2 {
		t.Errorf("GetLocalAddrs() count = %d, want 2", len(urls))
	}

	if urls[0] != "tcp://:19090" {
		t.Errorf("GetLocalAddrs()[0] = %q, want tcp://:19090", urls[0])
	}
	if urls[1] != "ipc://test" {
		t.Errorf("GetLocalAddrs()[1] = %q, want ipc://test", urls[1])
	}
}

// TestAgentServerStartStop tests Start and Stop methods
func TestAgentServerStartStop(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store) // Use :0 for random port

	// Test Stop when not running (should be no-op)
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop() when not running returned error: %v", err)
	}

	// Test Start
	err = server.Start()
	if err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	// Test Start when already running
	err = server.Start()
	if err == nil {
		t.Errorf("Start() when already running should return error")
	}

	// Test Stop
	err = server.Stop()
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify server is stopped
	if server.running {
		t.Errorf("server.running should be false after Stop()")
	}
}

// TestAgentServerStartError tests Start error handling
func TestAgentServerStartError(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer("invalid-address-format-!@#$", store)

	// Start should fail with invalid address
	err := server.Start()
	if err == nil {
		t.Errorf("Start() with invalid address should fail")
	}
}

// TestAgentServerStopMultiple tests calling Stop multiple times
func TestAgentServerStopMultiple(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	server.Start()

	// First stop
	err1 := server.Stop()
	// Second stop
	err2 := server.Stop()

	if err1 != nil {
		t.Errorf("First Stop() failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Stop() failed: %v", err2)
	}
}

// TestHandleInvoke tests handleInvoke method
func TestHandleInvoke(t *testing.T) {
	tests := []struct {
		name          string
		setupPM       bool
		setupStore    bool
		functionID    string
		expectError   bool
		errorContains string
	}{
		{
			name:        "Provider function call",
			setupPM:     true,
			setupStore:  false,
			functionID:  "platform.test",
			expectError: false,
		},
		{
			name:          "No provider manager but has store",
			setupPM:       false,
			setupStore:    true,
			functionID:    "test.function",
			expectError:   true,
			errorContains: "is not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := agentlocal.NewLocalStore()
			server := NewAgentServer(":0", store)

			if tt.setupPM {
				pm := &mockProviderManager{
					isPlatformFunc: func(id string) bool { return true },
					callFunc: func(ctx context.Context, id string, req []byte) ([]byte, error) {
						return []byte("response"), nil
					},
				}
				server.SetProviderManager(pm)
			}

			ctx := context.Background()
			req := &sdkv1.InvokeRequest{
				FunctionId: tt.functionID,
				Payload:    []byte("test payload"),
			}

			data, err := proto.Marshal(req)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			resp, err := server.handleInvoke(ctx, data)

			if tt.expectError {
				if err == nil {
					t.Errorf("handleInvoke() expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("handleInvoke() error = %v, want containing %q", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("handleInvoke() unexpected error: %v", err)
				}
				if resp == nil {
					t.Errorf("handleInvoke() response should not be nil on success")
				}
			}
		})
	}
}

// TestInvokePlatform tests invokePlatform method
func TestInvokePlatform(t *testing.T) {
	pm := &mockProviderManager{
		callFunc: func(ctx context.Context, functionID string, request []byte) ([]byte, error) {
			return []byte("platform response"), nil
		},
	}

	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)
	server.SetProviderManager(pm)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{
		FunctionId: "platform.test",
		Payload:    []byte("test"),
	}

	resp, err := server.invokePlatform(ctx, "platform.test", req)
	if err != nil {
		t.Errorf("invokePlatform() failed: %v", err)
	}

	response := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if string(response.Payload) != "platform response" {
		t.Errorf("invokePlatform() payload = %q, want 'platform response'", string(response.Payload))
	}
}

// TestPickInstance tests pickInstance method
func TestPickInstance(t *testing.T) {
	tests := []struct {
		name          string
		setupStore    bool
		functionID    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "No store initialized",
			setupStore:    false,
			functionID:    "test.function",
			expectError:   true,
			errorContains: "instance store not initialized",
		},
		{
			name:          "Empty function ID",
			setupStore:    true,
			functionID:    "",
			expectError:   true,
			errorContains: "function ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := agentlocal.NewLocalStore()
			server := NewAgentServer(":0", store)

			if !tt.setupStore {
				server.store = nil
			}

			addr, err := server.pickInstance(tt.functionID, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("pickInstance() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("pickInstance() error = %v, want containing %q", err, tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("pickInstance() unexpected error: %v", err)
				}
				if addr == "" {
					t.Errorf("pickInstance() address should not be empty on success")
				}
			}
		})
	}
}

// TestHandleStartJob tests handleStartJob method
func TestHandleStartJob(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	req := &sdkv1.InvokeRequest{
		FunctionId: "test.function",
		Payload:    []byte("test"),
	}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	resp, err := server.handleStartJob(ctx, data)
	if err != nil {
		t.Errorf("handleStartJob() failed: %v", err)
	}

	response := &sdkv1.StartJobResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if response.JobId == "" {
		t.Errorf("handleStartJob() job ID should not be empty")
	}
}

// TestHandleCancelJob tests handleCancelJob method
func TestHandleCancelJob(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()

	// First create a job
	startReq := &sdkv1.InvokeRequest{FunctionId: "test"}
	startData, _ := proto.Marshal(startReq)
	startResp, _ := server.handleStartJob(ctx, startData)

	startResponse := &sdkv1.StartJobResponse{}
	proto.Unmarshal(startResp, startResponse)
	jobID := startResponse.JobId

	// Now cancel it
	cancelReq := &sdkv1.CancelJobRequest{JobId: jobID}
	cancelData, _ := proto.Marshal(cancelReq)

	resp, err := server.handleCancelJob(ctx, cancelData)
	if err != nil {
		t.Errorf("handleCancelJob() failed: %v", err)
	}

	response := &sdkv1.StartJobResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	// Verify job was deleted
	if _, ok := server.jobs.Get(jobID); ok {
		t.Errorf("handleCancelJob() job should be deleted")
	}
}

// TestHandleGetSystemInfo tests handleGetSystemInfo with no ops server
func TestHandleGetSystemInfoNoOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store) // No ops server set

	ctx := context.Background()
	req := &emptypb.Empty{}
	data, _ := proto.Marshal(req)

	_, err := server.handleGetSystemInfo(ctx, data)
	if err == nil {
		t.Errorf("handleGetSystemInfo() should fail when ops server not configured")
	}
}

// TestHandleGetSystemInfoSuccess tests handleGetSystemInfo with ops server
func TestHandleGetSystemInfoSuccess(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		getSystemInfoFunc: func(ctx context.Context, req *emptypb.Empty) (*opsv1.SystemInfo, error) {
			return &opsv1.SystemInfo{
				Hostname: "test-host",
				Os:       "linux",
			}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &emptypb.Empty{}
	data, _ := proto.Marshal(req)

	resp, err := server.handleGetSystemInfo(ctx, data)
	if err != nil {
		t.Errorf("handleGetSystemInfo() failed: %v", err)
	}

	response := &opsv1.SystemInfo{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if response.Hostname != "test-host" {
		t.Errorf("handleGetSystemInfo() hostname = %q, want 'test-host'", response.Hostname)
	}
}

// TestHandleListProcesses tests handleListProcesses
func TestHandleListProcessesSuccess(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		listProcessesFunc: func(ctx context.Context, req *emptypb.Empty) (*opsv1.ListProcessesResponse, error) {
			return &opsv1.ListProcessesResponse{
				Processes: []*opsv1.ManagedProcess{
					{Name: "init", Pid: 1},
				},
			}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &emptypb.Empty{}
	data, _ := proto.Marshal(req)

	resp, err := server.handleListProcesses(ctx, data)
	if err != nil {
		t.Errorf("handleListProcesses() failed: %v", err)
	}

	response := &opsv1.ListProcessesResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if len(response.Processes) != 1 {
		t.Errorf("handleListProcesses() processes count = %d, want 1", len(response.Processes))
	}
}

// TestHandleListProcessesNoOpsServer tests handleListProcesses without ops server
func TestHandleListProcessesNoOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	req := &emptypb.Empty{}
	data, _ := proto.Marshal(req)

	_, err := server.handleListProcesses(ctx, data)
	if err == nil {
		t.Errorf("handleListProcesses() should fail when ops server not configured")
	}
}

// TestHandleReportMetrics tests handleReportMetrics without ops server
func TestHandleReportMetricsNoOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store) // No ops server set

	ctx := context.Background()
	req := &opsv1.MetricsReport{
		AgentId: "test-agent",
	}
	data, _ := proto.Marshal(req)

	resp, err := server.handleReportMetrics(ctx, data)
	if err != nil {
		t.Errorf("handleReportMetrics() should not fail when ops server not configured: %v", err)
	}

	response := &emptypb.Empty{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestHandleRestartProcess tests handleRestartProcess
func TestHandleRestartProcess(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		restartProcessFunc: func(ctx context.Context, req *opsv1.RestartProcessRequest) (*opsv1.RestartProcessResponse, error) {
			return &opsv1.RestartProcessResponse{Success: true}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &opsv1.RestartProcessRequest{ProcessName: "test-process"}
	data, _ := proto.Marshal(req)

	resp, err := server.handleRestartProcess(ctx, data)
	if err != nil {
		t.Errorf("handleRestartProcess() failed: %v", err)
	}

	response := &opsv1.RestartProcessResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestHandleStopProcess tests handleStopProcess
func TestHandleStopProcess(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		stopProcessFunc: func(ctx context.Context, req *opsv1.StopProcessRequest) (*opsv1.StopProcessResponse, error) {
			return &opsv1.StopProcessResponse{Success: true}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &opsv1.StopProcessRequest{ProcessName: "test-process"}
	data, _ := proto.Marshal(req)

	resp, err := server.handleStopProcess(ctx, data)
	if err != nil {
		t.Errorf("handleStopProcess() failed: %v", err)
	}

	response := &opsv1.StopProcessResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestHandleStartProcess tests handleStartProcess
func TestHandleStartProcess(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		startProcessFunc: func(ctx context.Context, req *opsv1.StartProcessRequest) (*opsv1.StartProcessResponse, error) {
			return &opsv1.StartProcessResponse{Success: true}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &opsv1.StartProcessRequest{ProcessName: "test-process"}
	data, _ := proto.Marshal(req)

	resp, err := server.handleStartProcess(ctx, data)
	if err != nil {
		t.Errorf("handleStartProcess() failed: %v", err)
	}

	response := &opsv1.StartProcessResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestHandleExecuteCommand tests handleExecuteCommand
func TestHandleExecuteCommand(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ops := &mockOpsServer{
		executeCommandFunc: func(ctx context.Context, req *opsv1.ExecuteCommandRequest) (*opsv1.ExecuteCommandResponse, error) {
			return &opsv1.ExecuteCommandResponse{ExitCode: 0, StdOut: "output"}, nil
		},
	}
	server.SetOpsServer(ops)

	ctx := context.Background()
	req := &opsv1.ExecuteCommandRequest{Command: "echo test"}
	data, _ := proto.Marshal(req)

	resp, err := server.handleExecuteCommand(ctx, data)
	if err != nil {
		t.Errorf("handleExecuteCommand() failed: %v", err)
	}

	response := &opsv1.ExecuteCommandResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestHandleRegisterLocal tests handleRegisterLocal
func TestHandleRegisterLocal(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	req := &sdkv1.RegisterLocalRequest{
		ServiceId: "provider1",
		RpcAddr:   "localhost:19090",
		Version:   "1.0.0",
		Functions: []*sdkv1.LocalFunctionDescriptor{
			{Id: "test.function1"},
			{Id: "test.function2"},
		},
	}
	data := sdkv1.MarshalRegisterLocalRequest(req)

	resp, err := server.handleRegisterLocal(ctx, data)
	if err != nil {
		t.Errorf("handleRegisterLocal() failed: %v", err)
	}

	response := sdkv1.UnmarshalRegisterLocalResponse(resp)
	if response == nil {
		t.Errorf("failed to unmarshal response")
	}

	// Verify function was registered
	snap := store.List()
	if _, ok := snap["test.function1"]; !ok {
		t.Errorf("handleRegisterLocal() function1 not registered")
	}
}

// TestHandleRegisterLocalNoStore tests handleRegisterLocal without store
func TestHandleRegisterLocalNoStore(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)
	server.store = nil

	ctx := context.Background()
	req := &sdkv1.RegisterLocalRequest{}
	data := sdkv1.MarshalRegisterLocalRequest(req)

	_, err := server.handleRegisterLocal(ctx, data)
	if err == nil {
		t.Errorf("handleRegisterLocal() should fail when store not initialized")
	}
}

// TestHandleHeartbeatLocal tests handleHeartbeatLocal
func TestHandleHeartbeatLocal(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	// First register
	regReq := &sdkv1.RegisterLocalRequest{
		ServiceId: "provider1",
		RpcAddr:   "localhost:19090",
		Version:   "1.0.0",
		Functions: []*sdkv1.LocalFunctionDescriptor{
			{Id: "test.function1"},
		},
	}
	regData := sdkv1.MarshalRegisterLocalRequest(regReq)
	_, _ = server.handleRegisterLocal(context.Background(), regData)

	ctx := context.Background()
	req := &sdkv1.HeartbeatRequest{ServiceId: "provider1"}
	data := sdkv1.MarshalHeartbeatRequestCompat(req)

	resp, err := server.handleHeartbeatLocal(ctx, data)
	if err != nil {
		t.Errorf("handleHeartbeatLocal() failed: %v", err)
	}

	// HeartbeatResponse is an empty message, no need to unmarshal
	_ = resp
}

// TestHandleHeartbeatLocalNoStore tests handleHeartbeatLocal without store
func TestHandleHeartbeatLocalNoStore(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)
	server.store = nil

	ctx := context.Background()
	req := &sdkv1.HeartbeatRequest{}
	data := sdkv1.MarshalHeartbeatRequestCompat(req)

	_, err := server.handleHeartbeatLocal(ctx, data)
	if err == nil {
		t.Errorf("handleHeartbeatLocal() should fail when store not initialized")
	}
}

// TestHandleListLocal tests handleListLocal
func TestHandleListLocal(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	// First register a provider
	regReq := &sdkv1.RegisterLocalRequest{
		ServiceId: "provider1",
		RpcAddr:   "localhost:19090",
		Version:   "1.0.0",
		Functions: []*sdkv1.LocalFunctionDescriptor{
			{Id: "test.function1"},
			{Id: "test.function2"},
		},
	}
	regData := sdkv1.MarshalRegisterLocalRequest(regReq)
	_, _ = server.handleRegisterLocal(context.Background(), regData)

	ctx := context.Background()
	data := []byte{}

	resp, err := server.handleListLocal(ctx, data)
	if err != nil {
		t.Errorf("handleListLocal() failed: %v", err)
	}

	response := sdkv1.UnmarshalListLocalResponse(resp)

	if len(response.Functions) != 2 {
		t.Errorf("handleListLocal() functions count = %d, want 2", len(response.Functions))
	}
}

// TestHandleListLocalNoStore tests handleListLocal without store
func TestHandleListLocalNoStore(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)
	server.store = nil

	ctx := context.Background()
	data := []byte{}

	_, err := server.handleListLocal(ctx, data)
	if err == nil {
		t.Errorf("handleListLocal() should fail when store not initialized")
	}
}

// TestHandleListServices tests handleListServices without ops server
func TestHandleListServicesNoOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	data := []byte("{}")

	_, err := server.handleListServices(ctx, data)
	if err == nil {
		t.Errorf("handleListServices() should fail when ops server not configured")
	}
}

// TestHandleGetServiceStatus tests handleGetServiceStatus without ops server
func TestHandleGetServiceStatusNoOpsServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	data := []byte("{}")

	_, err := server.handleGetServiceStatus(ctx, data)
	if err == nil {
		t.Errorf("handleGetServiceStatus() should fail when ops server not configured")
	}
}

// TestHandleRegisterCapabilities tests handleRegisterCapabilities
func TestHandleRegisterCapabilities(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	ctx := context.Background()
	req := &agentv1.RegisterCapabilitiesRequest{}
	data, _ := proto.Marshal(req)

	resp, err := server.handleRegisterCapabilities(ctx, data)
	if err != nil {
		t.Errorf("handleRegisterCapabilities() failed: %v", err)
	}

	response := &agentv1.RegisterCapabilitiesResponse{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}

// TestCreateErrorResponseAgentServer tests createErrorResponse method
func TestCreateErrorResponseAgentServer(t *testing.T) {
	store := agentlocal.NewLocalStore()
	server := NewAgentServer(":0", store)

	err := errors.New("test error")
	resp := server.createErrorResponse(err)

	if resp == nil {
		t.Errorf("createErrorResponse() should not return nil")
	}

	response := &emptypb.Empty{}
	if err := proto.Unmarshal(resp, response); err != nil {
		t.Errorf("failed to unmarshal error response: %v", err)
	}
}

// TestJobIndex tests jobIndex methods
func TestJobIndex(t *testing.T) {
	jobs := newJobIndex()

	// Test Set and Get
	jobs.Set("job1", "addr1")
	addr, ok := jobs.Get("job1")
	if !ok {
		t.Errorf("jobIndex.Get() failed to find job1")
	}
	if addr != "addr1" {
		t.Errorf("jobIndex.Get() = %q, want 'addr1'", addr)
	}

	// Test Get non-existent
	_, ok = jobs.Get("job2")
	if ok {
		t.Errorf("jobIndex.Get() should not find non-existent job")
	}

	// Test Delete
	jobs.Delete("job1")
	_, ok = jobs.Get("job1")
	if ok {
		t.Errorf("jobIndex.Get() should not find deleted job")
	}

	// Test concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			jobs.Set("job-concurrent", "addr")
			jobs.Get("job-concurrent")
			jobs.Delete("job-concurrent")
		}(i)
	}
	wg.Wait()
}

// TestHostFromAddr tests hostFromAddr function
func TestHostFromAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"Simple host:port", "localhost:19090", "localhost"},
		{"IPv4 address", "192.168.1.1:8080", "192.168.1.1"},
		{"IPv6 address", "[::1]:19090", "::1"},
		{"IPv6 address full", "[2001:db8::1]:8080", "2001:db8::1"},
		{"Empty string", "", ""},
		{"Just host", "localhost", "localhost"},
		{"Just IPv6", "::1", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostFromAddr(tt.addr)
			if got != tt.want {
				t.Errorf("hostFromAddr(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

// TestFnvIndex tests fnvIndex function
func TestFnvIndex(t *testing.T) {
	tests := []struct {
		name string
		key  string
		mod  int
	}{
		{"Simple key", "test", 10},
		{"Empty key", "", 10},
		{"Mod 1", "test", 1},
		{"Mod 0", "test", 0},
		{"Large mod", "test", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fnvIndex(tt.key, tt.mod)

			if tt.mod <= 1 {
				if result != 0 {
					t.Errorf("fnvIndex() mod=%d should return 0, got %d", tt.mod, result)
				}
			} else {
				if result < 0 || result >= tt.mod {
					t.Errorf("fnvIndex() = %d, want in range [0, %d)", result, tt.mod)
				}
			}
		})
	}

	// Test consistency - same key should give same result
	result1 := fnvIndex("test-key", 100)
	result2 := fnvIndex("test-key", 100)
	if result1 != result2 {
		t.Errorf("fnvIndex() not consistent: %d != %d", result1, result2)
	}

	// Test different keys likely give different results
	result3 := fnvIndex("different-key", 100)
	if result1 == result3 {
		t.Logf("fnvIndex() collision (acceptable but unlikely)")
	}
}
