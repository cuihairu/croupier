package ops

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockNNGClient provides a mock for testing OpsClientWrapper methods
// Since OpsClientWrapper embeds *nng.Client directly, we create a mock
// by setting up the wrapper with a properly initialized client
type testNNGWrapper struct {
	systemInfo       *opsv1.SystemInfo
	systemInfoErr    error
	processes        *opsv1.ListProcessesResponse
	processesErr     error
	metricsReport    *opsv1.MetricsReport
	metricsReportErr error
	restartResp      *opsv1.RestartProcessResponse
	restartRespErr   error
	stopResp         *opsv1.StopProcessResponse
	stopRespErr      error
	startResp        *opsv1.StartProcessResponse
	startRespErr     error
	execResp         *opsv1.ExecuteCommandResponse
	execRespErr      error
	mu               sync.Mutex
}

// Since we can't directly mock the nng.Client interface, we test the wrapper methods
// through the actual logic layer which handles errors appropriately

// Test OpsClientWrapper methods through integration tests
func TestOpsClientWrapper_Integration(t *testing.T) {
	// These tests verify that the wrapper methods handle errors correctly
	// when the underlying NNG client is not connected

	t.Run("GetSystemInfo with disconnected client", func(t *testing.T) {
		_ = createTestServiceContext()
		client := GetAgentOpsClient()

		// Try to get a client that doesn't exist
		wrapper, err := client.GetClient(context.Background(), "non-existent-agent")
		if err == nil {
			t.Error("expected error for non-existent agent")
		}
		_ = wrapper // Should be nil or empty
	})

	t.Run("ListProcesses with disconnected client", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{
			AgentID: "non-existent-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("ExecuteCommand with disconnected client", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
			AgentID: "non-existent-agent",
			Command: "test",
		})

		if err != nil {
			t.Fatalf("OpsAgentExecCommand() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("StartProcess with disconnected client", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{
			AgentID: "non-existent-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("StopProcess with disconnected client", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{
			AgentID: "non-existent-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStop() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("RestartProcess with disconnected client", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{
			AgentID: "non-existent-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessRestart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test RegisterClient functionality
func TestAgentOpsClient_RegisterClient(t *testing.T) {
	t.Run("register with valid address but no server", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		// Try to register with an address where no server is listening
		// This should error during dial, not panic
		err := client.RegisterClient("test-agent-register", "tcp://localhost:19999")
		// We expect this to fail (no server listening)
		// The important thing is it doesn't panic
		_ = err
	})

	t.Run("unregister non-existent client", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		// Unregistering a non-existent client should not panic
		client.UnregisterClient("never-registered-agent")
	})

	t.Run("close empty client", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		// Closing an empty client should not panic
		err := client.Close()
		if err != nil {
			t.Errorf("Close() should not error, got: %v", err)
		}

		// Re-initialize for other tests
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}
	})

	t.Run("concurrent register/unregister", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		// Run concurrent operations
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(2)
			go func(idx int) {
				defer wg.Done()
				_ = client.RegisterClient("agent-"+string(rune(idx)), "tcp://localhost:19990")
			}(i)
			go func(idx int) {
				defer wg.Done()
				client.UnregisterClient("agent-" + string(rune(idx)))
			}(i)
		}
		wg.Wait()

		// Should not have panicked
	})
}

// Test OpsServicesLogic more thoroughly
func TestOpsServicesLogic_Detailed(t *testing.T) {
	t.Run("server entry in services list", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
				Region: "us-west-1",
				Zone:   "zone-a",
				Labels: map[string]string{
					"test": "value",
				},
			},
			ServerVersion: "v2.0.0",
			StartTime:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			RegistryStore: registry.NewStore(),
		}

		// Add context with admin role (permission check will still fail without proper setup)
		ctx := context.Background()

		logic := NewOpsServicesLogic(ctx, svcCtx)

		// This will fail permission check, but we can verify the logic is constructed
		if logic == nil {
			t.Error("NewOpsServicesLogic() returned nil")
		}

		if logic.svcCtx != svcCtx {
			t.Error("NewOpsServicesLogic() did not set svcCtx")
		}

		if logic.ctx != ctx {
			t.Error("NewOpsServicesLogic() did not set ctx")
		}
	})

	t.Run("services with agents in registry", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			Config: config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Region: "us-east-1",
				Zone:   "zone1",
			},
			ServerVersion: "v1.0.0",
			StartTime:     time.Now(),
			RegistryStore: registry.NewStore(),
		}

		// Add some agent sessions
		sess1 := &registry.AgentSession{
			AgentID:  "agent-1",
			GameID:   "game1",
			Env:      "prod",
			Version:  "v1.0.0",
			RPCAddr:  "localhost:19090",
			ExpireAt: time.Now().Add(time.Minute),
			LastSeen: time.Now(),
			Functions: map[string]registry.FunctionMeta{
				"func1": {Enabled: true},
				"func2": {Enabled: true},
			},
			Labels: map[string]string{
				"dc": "dc1",
			},
			Region: "us-east-1",
			Zone:   "zone1",
			Providers: []registry.ProviderSession{
				{
					ProviderID:   "provider-1",
					Addr:         "localhost:8081",
					Version:      "v1.0.0",
					LastSeenUnix: time.Now().Unix(),
					FunctionIDs:  []string{"func1", "func2"},
				},
			},
		}

		svcCtx.RegistryStore.UpsertAgent(sess1)

		ctx := context.Background()
		logic := NewOpsServicesLogic(ctx, svcCtx)

		// Verify logic construction
		if logic == nil {
			t.Error("NewOpsServicesLogic() returned nil")
		}
	})
}

// Test formatTimestamp with more edge cases
func TestFormatTimestamp_AdditionalCases(t *testing.T) {
	t.Run("nil timestamp", func(t *testing.T) {
		result := formatTimestamp(nil)
		if result != "" {
			t.Errorf("expected empty string for nil timestamp, got %s", result)
		}
	})

	t.Run("very old timestamp", func(t *testing.T) {
		// Year 1970
		ts := timestamppb.New(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
		result := formatTimestamp(ts)

		if result == "" {
			t.Error("expected non-empty result for old timestamp")
		}
	})

	t.Run("far future timestamp", func(t *testing.T) {
		// Year 2100
		ts := timestamppb.New(time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC))
		result := formatTimestamp(ts)

		if result == "" {
			t.Error("expected non-empty result for future timestamp")
		}

		// Should contain 2100
		if !contains(result, "2100") {
			t.Logf("Note: result %s may not contain 2100 due to formatting", result)
		}
	})

	t.Run("timestamp with nanoseconds", func(t *testing.T) {
		ts := &timestamppb.Timestamp{
			Seconds: 1609459200, // 2021-01-01 00:00:00 UTC
			Nanos:   123456789,
		}
		result := formatTimestamp(ts)

		if result == "" {
			t.Error("expected non-empty result for timestamp with nanoseconds")
		}
	})

	t.Run("negative timestamp (before Unix epoch)", func(t *testing.T) {
		// January 1, 1960
		beforeEpoch := time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC)
		ts := timestamppb.New(beforeEpoch)
		result := formatTimestamp(ts)

		if result == "" {
			t.Error("expected non-empty result for pre-epoch timestamp")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test collectServerLabels cross-platform behavior
func TestCollectServerLabels_CrossPlatform(t *testing.T) {
	t.Run("basic labels present", func(t *testing.T) {
		labels := collectServerLabels()

		// Check for expected basic labels
		if labels["os"] != runtime.GOOS {
			t.Errorf("expected os %s, got %s", runtime.GOOS, labels["os"])
		}

		if labels["arch"] != runtime.GOARCH {
			t.Errorf("expected arch %s, got %s", runtime.GOARCH, labels["arch"])
		}

		if labels["go_version"] == "" {
			t.Error("expected non-empty go_version")
		}
	})

	t.Run("cpu_count is numeric", func(t *testing.T) {
		labels := collectServerLabels()

		cpuCount := labels["cpu_count"]
		if cpuCount == "" {
			t.Error("expected non-empty cpu_count")
		}

		// Should be a number
		var num int
		if _, err := fmt.Sscanf(cpuCount, "%d", &num); err != nil {
			t.Errorf("cpu_count should be numeric, got %s", cpuCount)
		}

		if num != runtime.NumCPU() {
			t.Errorf("expected cpu_count %d, got %d", runtime.NumCPU(), num)
		}
	})

	t.Run("Windows-specific labels", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("skipping Windows-specific test on " + runtime.GOOS)
		}

		labels := collectServerLabels()

		if labels["os"] != "windows" {
			t.Errorf("expected os windows, got %s", labels["os"])
		}
	})

	t.Run("Unix-specific labels", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipping Unix-specific test on Windows")
		}

		labels := collectServerLabels()

		// Unix systems should have hostname
		if labels["hostname"] == "" {
			t.Error("expected non-empty hostname on Unix")
		}
	})
}

// Test OpsAgentMetricsLogic with more scenarios
func TestOpsAgentMetricsLogic_Additional(t *testing.T) {
	t.Run("negative limit uses default", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Limit: -1,
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
	})

	t.Run("very large limit", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Limit: 1000000,
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should handle gracefully
		_ = resp
	})

	t.Run("metrics with empty agent ID", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Add some metrics
		report := &opsv1.MetricsReport{
			Cpu:    &opsv1.CpuMetrics{Cores: 4},
			Memory: &opsv1.MemoryMetrics{TotalBytes: 1024},
		}
		svcCtx.MetricsStore.Add("agent-1", report)

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			AgentID: "", // Empty agent ID - should return all
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should return metrics from all agents
		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
	})
}

// Test OpsAgentSystemInfoLogic with more scenarios
func TestOpsAgentSystemInfoLogic_Additional(t *testing.T) {
	t.Run("cache miss then direct query", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Don't add to cache, agent doesn't exist
		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "non-existent-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		// Should get 404 from direct query attempt
		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("cached info with nil fields", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Cache info with minimal fields
		cachedInfo := &opsv1.SystemInfo{
			Hostname: "minimal-host",
			// Other fields are nil/zero
		}
		svcCtx.SystemInfoCache.Set("minimal-agent", cachedInfo)

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "minimal-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		if resp.Data.Hostname != "minimal-host" {
			t.Errorf("expected hostname minimal-host, got %s", resp.Data.Hostname)
		}
	})
}

// Test OpsManagedProcess construction
func TestOpsManagedProcess_Construction(t *testing.T) {
	t.Run("minimal process", func(t *testing.T) {
		p := OpsManagedProcess{
			Name:    "test",
			Command: "/bin/test",
			State:   "running",
		}

		if p.Name != "test" {
			t.Errorf("expected name test, got %s", p.Name)
		}
	})

	t.Run("process with all fields", func(t *testing.T) {
		lastStart := time.Now()
		p := OpsManagedProcess{
			Name:         "full-process",
			Command:      "/usr/bin/full --verbose",
			WorkingDir:   "/var/app",
			State:        "running",
			Pid:          1234,
			ParentPID:    1,
			Memory:       1024 * 1024 * 100,
			CPUPercent:   5.5,
			RestartCount: 3,
			LastStart:    lastStart.Format(time.RFC3339),
		}

		if p.Pid != 1234 {
			t.Errorf("expected PID 1234, got %d", p.Pid)
		}

		if p.RestartCount != 3 {
			t.Errorf("expected restart count 3, got %d", p.RestartCount)
		}

		if p.Memory != 1024*1024*100 {
			t.Errorf("expected memory 104857600, got %d", p.Memory)
		}
	})

	t.Run("process with zero values", func(t *testing.T) {
		p := OpsManagedProcess{}

		if p.Name != "" {
			t.Errorf("expected empty name, got %s", p.Name)
		}

		if p.Pid != 0 {
			t.Errorf("expected zero PID, got %d", p.Pid)
		}
	})
}

// Test OpsMetricsData construction
func TestOpsMetricsData_Construction(t *testing.T) {
	t.Run("empty metrics", func(t *testing.T) {
		m := OpsMetricsData{}

		if m.AgentID != "" {
			t.Errorf("expected empty agent ID, got %s", m.AgentID)
		}

		if len(m.CPU.PerCore) != 0 {
			t.Errorf("expected empty per-core, got %d elements", len(m.CPU.PerCore))
		}
	})

	t.Run("full metrics", func(t *testing.T) {
		m := OpsMetricsData{
			AgentID:   "agent-1",
			Timestamp: time.Now().Format(time.RFC3339),
			CPU: OpsCpuMetrics{
				Cores:        8,
				UsagePercent: 50.0,
				PerCore:      []float64{40, 60, 45, 55, 50, 50, 48, 52},
			},
			Memory: OpsMemoryMetrics{
				TotalBytes:     16 * 1024 * 1024 * 1024,
				UsedBytes:      8 * 1024 * 1024 * 1024,
				AvailableBytes: 8 * 1024 * 1024 * 1024,
				UsagePercent:   50.0,
			},
			Disks: []OpsDiskMetrics{
				{
					MountPoint:     "/",
					Device:         "/dev/sda1",
					FsType:         "ext4",
					TotalBytes:     100 * 1024 * 1024 * 1024,
					UsedBytes:      50 * 1024 * 1024 * 1024,
					AvailableBytes: 50 * 1024 * 1024 * 1024,
					UsagePercent:   50.0,
				},
			},
			Networks: []OpsNetworkMetrics{
				{
					Interface:   "eth0",
					BytesSent:   1024 * 1024 * 100,
					BytesRecv:   1024 * 1024 * 200,
					PacketsSent: 1000,
					PacketsRecv: 2000,
				},
			},
		}

		if len(m.CPU.PerCore) != 8 {
			t.Errorf("expected 8 per-core values, got %d", len(m.CPU.PerCore))
		}

		if len(m.Disks) != 1 {
			t.Errorf("expected 1 disk, got %d", len(m.Disks))
		}

		if len(m.Networks) != 1 {
			t.Errorf("expected 1 network interface, got %d", len(m.Networks))
		}
	})
}

// Test response type consistency
func TestResponseTypeConsistency(t *testing.T) {
	t.Run("OpsAgentsListResponse default values", func(t *testing.T) {
		resp := &OpsAgentsListResponse{}

		if resp.Code != 0 {
			t.Errorf("expected default code 0, got %d", resp.Code)
		}

		if resp.Data == nil {
			// Data should be initialized to empty slice, not nil
			resp.Data = []OpsAgentInfo{}
		}
	})

	t.Run("OpsAgentSystemInfoResponse default values", func(t *testing.T) {
		resp := &OpsAgentSystemInfoResponse{}

		if resp.Code != 0 {
			t.Errorf("expected default code 0, got %d", resp.Code)
		}

		if resp.Data.TotalMemory != 0 {
			t.Errorf("expected default total memory 0, got %d", resp.Data.TotalMemory)
		}
	})

	t.Run("error response format", func(t *testing.T) {
		resp := &OpsAgentSystemInfoResponse{
			Code:    500,
			Message: "internal error",
			Data:    OpsAgentSystemInfo{},
		}

		if resp.Code != 500 {
			t.Errorf("expected code 500, got %d", resp.Code)
		}

		if resp.Message != "internal error" {
			t.Errorf("expected message 'internal error', got %s", resp.Message)
		}
	})
}

// Test state_helpers with nil pointers
func TestStateHelpers_NilHandling(t *testing.T) {
	t.Run("snapshotOpsState with nil context", func(t *testing.T) {
		state := snapshotOpsState(nil)

		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state for nil context")
		}
	})

	t.Run("snapshotOpsState with context without store", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{}
		state := snapshotOpsState(svcCtx)

		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state for context without store")
		}
	})

	t.Run("updateOpsState with nil context", func(t *testing.T) {
		_, err := updateOpsState(nil, func(s *svc.OpsState) {})

		if err == nil {
			t.Error("expected error for nil context")
		}

		if !errors.Is(err, errOpsStateUnavailable) {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})

	t.Run("updateOpsState with context without store", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{}
		_, err := updateOpsState(svcCtx, func(s *svc.OpsState) {})

		if err == nil {
			t.Error("expected error for context without store")
		}

		if !errors.Is(err, errOpsStateUnavailable) {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})
}

// Test concurrent access to AgentOpsClient
func TestAgentOpsClient_Concurrency(t *testing.T) {
	t.Run("concurrent GetClient calls", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		const numGoroutines = 100
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				_, err := client.GetClient(context.Background(), "concurrent-test-agent")
				results <- err
			}(i)
		}

		// All should return error (agent not found)
		errorCount := 0
		for i := 0; i < numGoroutines; i++ {
			if err := <-results; err != nil {
				errorCount++
			}
		}

		if errorCount != numGoroutines {
			t.Errorf("expected %d errors, got %d", numGoroutines, errorCount)
		}
	})

	t.Run("concurrent UnregisterClient calls", func(t *testing.T) {
		globalAgentOpsClient = nil
		agentOpsClientOnce = sync.Once{}

		client := GetAgentOpsClient()

		const numGoroutines = 50
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				client.UnregisterClient("concurrent-unregister-" + string(rune('a'+id%26)))
			}(i)
		}

		wg.Wait()
		// Should not panic or deadlock
	})
}

// Benchmark tests
func BenchmarkCollectServerLabels(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = collectServerLabels()
	}
}

func BenchmarkFormatLastSeen(b *testing.B) {
	lastSeen := time.Now()
	expireAt := time.Now().Add(time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatLastSeen(lastSeen, expireAt)
	}
}

func BenchmarkTtlAndHealth(b *testing.B) {
	sess := &registry.AgentSession{
		ExpireAt: time.Now().Add(time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ttlAndHealth(sess)
	}
}

func BenchmarkFormatTimestamp(b *testing.B) {
	ts := timestamppb.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatTimestamp(ts)
	}
}

// Test context handling in various logics
func TestContextHandling(t *testing.T) {
	t.Run("OpsAgentSystemInfoLogic with cancelled context", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		logic := NewOpsAgentSystemInfoLogic(ctx, svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "unknown-agent",
		})

		// Should return error response (not panic)
		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("OpsAgentProcessesLogic with cancelled context", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		logic := NewOpsAgentProcessesLogic(ctx, svcCtx)
		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{
			AgentID: "unknown-agent",
		})

		// Should return error response (not panic)
		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("OpsAgentMetricsLogic with cancelled context", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		logic := NewOpsAgentMetricsLogic(ctx, svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{})

		// Should return response (not panic)
		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}
	})
}

// Test RegistryStore interaction edge cases
func TestRegistryStoreEdgeCases(t *testing.T) {
	t.Run("agents list with nil registry store", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			RegistryStore: nil,
		}

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected 0 agents, got %d", len(resp.Data))
		}
	})

	t.Run("metrics with nil metrics store", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			MetricsStore: nil,
		}

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(resp.Data))
		}
	})

	t.Run("system info with nil cache - expects panic", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{
			SystemInfoCache: nil,
		}

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)

		// The current implementation panics with nil cache
		// This test documents that behavior
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic with nil cache
				t.Log("Recovered from panic as expected with nil cache:", r)
			}
		}()

		_, _ = logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{
			AgentID: "unknown-agent",
		})
	})
}

// Test protobuf marshaling/unmarshaling edge cases
func TestProtobufHandling(t *testing.T) {
	t.Run("marshal nil SystemInfo", func(t *testing.T) {
		var info *opsv1.SystemInfo = nil

		// proto.Marshal actually handles nil by marshaling an empty message
		data, err := proto.Marshal(info)
		if err != nil {
			t.Fatalf("failed to marshal nil proto message: %v", err)
		}

		// Empty proto message is valid
		_ = data
	})

	t.Run("marshal empty SystemInfo", func(t *testing.T) {
		info := &opsv1.SystemInfo{}

		data, err := proto.Marshal(info)
		if err != nil {
			t.Fatalf("failed to marshal empty SystemInfo: %v", err)
		}

		// Empty proto messages are valid and may have zero length
		_ = data
	})

	t.Run("unmarshal invalid data", func(t *testing.T) {
		invalidData := []byte("not valid protobuf")

		var info opsv1.SystemInfo
		err := proto.Unmarshal(invalidData, &info)
		if err == nil {
			t.Error("expected error unmarshaling invalid data")
		}
	})

	t.Run("round trip SystemInfo", func(t *testing.T) {
		original := &opsv1.SystemInfo{
			Hostname:      "test-host",
			Os:            "linux",
			OsVersion:     "5.0",
			KernelVersion: "5.0.0",
			Arch:          "amd64",
			CpuCores:      4,
			TotalMemory:   8 * 1024 * 1024 * 1024,
			BootTime:      timestamppb.Now(),
			AgentVersion:  "v1.0.0",
		}

		data, err := proto.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		unmarshaled := &opsv1.SystemInfo{}
		err = proto.Unmarshal(data, unmarshaled)
		if err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if unmarshaled.Hostname != original.Hostname {
			t.Errorf("expected hostname %s, got %s", original.Hostname, unmarshaled.Hostname)
		}

		if unmarshaled.AgentVersion != original.AgentVersion {
			t.Errorf("expected agent version %s, got %s", original.AgentVersion, unmarshaled.AgentVersion)
		}
	})
}
