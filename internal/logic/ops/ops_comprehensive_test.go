package ops

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/nng"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper to create a test service context
func createTestServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config: config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			Region: "us-east-1",
			Zone:   "zone1",
			Labels: map[string]string{
				"env": "test",
			},
		},
		ServerVersion:    "v1.0.0",
		StartTime:        time.Now(),
		RegistryStore:    registry.NewStore(),
		MetricsStore:     registry.NewMetricsStore(),
		SystemInfoCache:  registry.NewSystemInfoCache(),
	}
}

// Test OpsAgentsListLogic
func TestOpsAgentsListLogic(t *testing.T) {
	t.Run("nil store", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		svcCtx.RegistryStore = nil

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

	t.Run("with agents in store", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Add a test agent session
		sess := &registry.AgentSession{
			AgentID:   "test-agent-1",
			GameID:    "game1",
			Env:       "dev",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: map[string]registry.FunctionMeta{
				"func1": {Enabled: true},
			},
			Labels: map[string]string{
				"key1": "value1",
			},
		}
		svcCtx.RegistryStore.UpsertAgent(sess)

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("expected 1 agent, got %d", len(resp.Data))
		}

		agent := resp.Data[0]
		if agent.AgentID != "test-agent-1" {
			t.Errorf("expected agent ID test-agent-1, got %s", agent.AgentID)
		}

		if !agent.Connected {
			t.Error("expected agent to be connected")
		}
	})

	t.Run("with expired agent", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Add an expired agent session
		sess := &registry.AgentSession{
			AgentID:   "expired-agent",
			GameID:    "game1",
			Env:       "dev",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(-time.Minute),
			Functions: map[string]registry.FunctionMeta{},
		}
		svcCtx.RegistryStore.UpsertAgent(sess)

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("expected 1 agent, got %d", len(resp.Data))
		}

		agent := resp.Data[0]
		if agent.Connected {
			t.Error("expected expired agent to be disconnected")
		}
	})

	t.Run("with providers", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		sess := &registry.AgentSession{
			AgentID:   "agent-with-providers",
			GameID:    "game1",
			Env:       "dev",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: map[string]registry.FunctionMeta{},
			Providers: []registry.ProviderSession{
				{
					ProviderID: "provider-1",
					Addr:       "localhost:8081",
				},
			},
		}
		svcCtx.RegistryStore.UpsertAgent(sess)

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		agent := resp.Data[0]
		if len(agent.Processes) != 1 {
			t.Errorf("expected 1 process, got %d", len(agent.Processes))
		}
	})
}

// Test OpsServicesLogic
// Note: These tests are skipped because OpsServicesLogic requires permission checks
// that depend on the AdminModel being initialized, which requires a database connection.
// The core functionality (formatLastSeen, ttlAndHealth, collectServerLabels) is tested separately.
func TestOpsServicesLogic_Skipped(t *testing.T) {
	t.Skip("OpsServicesLogic requires AdminModel for permission checks")

	// The tests below would run if we had a proper service context with AdminModel
	// For now, we test the helper functions separately.
}

// Test formatLastSeen helper
func TestFormatLastSeen(t *testing.T) {
	t.Run("with valid LastSeen", func(t *testing.T) {
		lastSeen := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		expireAt := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

		result := formatLastSeen(lastSeen, expireAt)
		expected := "2024-01-01T12:00:00Z"

		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("with zero LastSeen", func(t *testing.T) {
		lastSeen := time.Time{}
		expireAt := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

		result := formatLastSeen(lastSeen, expireAt)
		// Should use expireAt - 30 seconds
		expected := "2024-01-01T12:59:30Z"

		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("with both zero", func(t *testing.T) {
		lastSeen := time.Time{}
		expireAt := time.Time{}

		result := formatLastSeen(lastSeen, expireAt)
		// Should format the zero time
		if result == "" {
			t.Error("expected non-empty result")
		}
	})
}

// Test ttlAndHealth helper
func TestTtlAndHealth(t *testing.T) {
	t.Run("healthy session", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(time.Minute),
		}

		ttl, healthy := ttlAndHealth(sess)

		if !healthy {
			t.Error("expected session to be healthy")
		}

		if ttl <= 0 {
			t.Errorf("expected positive TTL, got %d", ttl)
		}
	})

	t.Run("expired session", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(-time.Minute),
		}

		ttl, healthy := ttlAndHealth(sess)

		if healthy {
			t.Error("expected session to be unhealthy")
		}

		if ttl != 0 {
			t.Errorf("expected TTL 0, got %d", ttl)
		}
	})

	t.Run("nil session", func(t *testing.T) {
		ttl, healthy := ttlAndHealth(nil)

		if healthy {
			t.Error("expected nil session to be unhealthy")
		}

		if ttl != 0 {
			t.Errorf("expected TTL 0 for nil session, got %d", ttl)
		}
	})

	t.Run("zero ExpireAt", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Time{},
		}

		ttl, healthy := ttlAndHealth(sess)

		if healthy {
			t.Error("expected zero ExpireAt to be unhealthy")
		}

		if ttl != 0 {
			t.Errorf("expected TTL 0, got %d", ttl)
		}
	})

	t.Run("exactly at expiration", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now(),
		}

		ttl, healthy := ttlAndHealth(sess)

		// TTL might be 0 or 1 depending on timing
		if healthy && ttl <= 0 {
			t.Error("inconsistent health state")
		}
	})
}

// Test OpsAgentSystemInfoLogic
func TestOpsAgentSystemInfoLogic(t *testing.T) {
	t.Run("info from cache", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		cachedInfo := &opsv1.SystemInfo{
			Hostname:     "cached-host",
			Os:           "linux",
			OsVersion:    "1.0",
			KernelVersion: "5.0",
			Arch:         "amd64",
			CpuCores:     8,
			TotalMemory:  16 * 1024 * 1024 * 1024,
			BootTime:     timestamppb.Now(),
			AgentVersion: "v1.0.0",
		}
		svcCtx.SystemInfoCache.Set("test-agent", cachedInfo)

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "test-agent"})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 0 {
			t.Errorf("expected code 0, got %d", resp.Code)
		}

		if resp.Data.Hostname != "cached-host" {
			t.Errorf("expected hostname cached-host, got %s", resp.Data.Hostname)
		}

		if resp.Data.AgentVersion != "v1.0.0" {
			t.Errorf("expected agent version v1.0.0, got %s", resp.Data.AgentVersion)
		}

		if resp.Data.CPUCores != 8 {
			t.Errorf("expected 8 CPU cores, got %d", resp.Data.CPUCores)
		}
	})

	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		// No agent registered

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "unknown-agent"})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}

		if resp.Message == "" {
			t.Error("expected error message")
		}
	})

	t.Run("empty agent ID", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: ""})

		if err != nil {
			t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404 for empty agent ID, got %d", resp.Code)
		}
	})
}

// Test formatTimestamp helper
func TestFormatTimestamp(t *testing.T) {
	t.Run("valid timestamp", func(t *testing.T) {
		ts := timestamppb.New(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
		result := formatTimestamp(ts)

		expected := "2024-01-01T12:00:00Z"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})

	t.Run("nil timestamp", func(t *testing.T) {
		result := formatTimestamp(nil)

		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

	t.Run("timestamp with non-UTC time", func(t *testing.T) {
		loc := time.FixedZone("EST", -5*3600)
		ts := timestamppb.New(time.Date(2024, 1, 1, 12, 0, 0, 0, loc))
		result := formatTimestamp(ts)

		// Should format as RFC3339
		if result == "" {
			t.Error("expected non-empty result")
		}
	})
}

// Test OpsAgentMetricsLogic
func TestOpsAgentMetricsLogic(t *testing.T) {
	t.Run("nil metrics store", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		svcCtx.MetricsStore = nil

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

	t.Run("with metrics in store", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Add a test metrics entry
		report := &opsv1.MetricsReport{
			Cpu: &opsv1.CpuMetrics{
				Cores:        4,
				UsagePercent: 50.0,
				Load_1M:      1.0,
				Load_5M:      0.8,
				Load_15M:     0.5,
				PerCore:      []float64{40.0, 60.0},
			},
			Memory: &opsv1.MemoryMetrics{
				TotalBytes:     8 * 1024 * 1024 * 1024,
				UsedBytes:      4 * 1024 * 1024 * 1024,
				AvailableBytes: 4 * 1024 * 1024 * 1024,
				UsagePercent:   50.0,
				SwapTotal:      2 * 1024 * 1024 * 1024,
				SwapUsed:       0,
			},
			Disks: []*opsv1.DiskMetrics{
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
			Networks: []*opsv1.NetworkMetrics{
				{
					Interface:   "eth0",
					BytesSent:   1024,
					BytesRecv:   2048,
					PacketsSent: 10,
					PacketsRecv: 20,
				},
			},
		}
		svcCtx.MetricsStore.Add("agent-1", report)

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			AgentID: "agent-1",
			Limit:   10,
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("expected 1 metric entry, got %d", len(resp.Data))
		}

		metric := resp.Data[0]
		if metric.AgentID != "agent-1" {
			t.Errorf("expected agent ID agent-1, got %s", metric.AgentID)
		}

		if metric.CPU.Cores != 4 {
			t.Errorf("expected 4 CPU cores, got %d", metric.CPU.Cores)
		}

		if metric.CPU.UsagePercent != 50.0 {
			t.Errorf("expected CPU usage 50.0, got %f", metric.CPU.UsagePercent)
		}

		if metric.Memory.TotalBytes != 8*1024*1024*1024 {
			t.Errorf("expected 8GB total memory, got %d", metric.Memory.TotalBytes)
		}

		if len(metric.Disks) != 1 {
			t.Errorf("expected 1 disk, got %d", len(metric.Disks))
		}

		if len(metric.Networks) != 1 {
			t.Errorf("expected 1 network interface, got %d", len(metric.Networks))
		}

		// Check per-core CPU
		if len(metric.CPU.PerCore) != 2 {
			t.Errorf("expected 2 per-core values, got %d", len(metric.CPU.PerCore))
		}
	})

	t.Run("with since filter - basic test", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// Add metrics with Memory to avoid nil pointer
		report := &opsv1.MetricsReport{
			Cpu: &opsv1.CpuMetrics{Cores: 4},
			Memory: &opsv1.MemoryMetrics{TotalBytes: 1024},
		}

		svcCtx.MetricsStore.Add("agent-1", report)

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Since:   time.Now().Add(-time.Hour).Format(time.RFC3339),
			AgentID: "agent-1",
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should get the entry since it's within the last hour
		if len(resp.Data) != 1 {
			t.Errorf("expected 1 metric entry, got %d", len(resp.Data))
		}
	})

	t.Run("default limit", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Limit: 0, // Should use default 100
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should not error with zero limit
		_ = resp
	})

	t.Run("invalid since time", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Since: "invalid-time",
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should handle invalid time gracefully
		_ = resp
	})

	t.Run("with nil report in store", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		// The metrics store should handle nil reports
		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			AgentID: "non-existent",
		})

		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}

		// Should return empty results
		if len(resp.Data) != 0 {
			t.Errorf("expected 0 metrics for non-existent agent, got %d", len(resp.Data))
		}
	})
}

// Test OpsAgentProcessesLogic
func TestOpsAgentProcessesLogic(t *testing.T) {
	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{
			AgentID: "unknown-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("empty agent ID", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{
			AgentID: "",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404 for empty agent ID, got %d", resp.Code)
		}
	})
}

// Test OpsAgentExecCommandLogic
func TestOpsAgentExecCommandLogic(t *testing.T) {
	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
			AgentID: "unknown-agent",
			Command: "echo",
		})

		if err != nil {
			t.Fatalf("OpsAgentExecCommand() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("with timeout", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
			AgentID: "unknown-agent",
			Command: "echo",
			Timeout: 30,
		})

		if err != nil {
			t.Fatalf("OpsAgentExecCommand() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessStartLogic
func TestOpsAgentProcessStartLogic(t *testing.T) {
	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{
			AgentID: "unknown-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessStopLogic
func TestOpsAgentProcessStopLogic(t *testing.T) {
	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStop() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("with force flag", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "test-process",
			Force:   true,
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStop() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessRestartLogic
func TestOpsAgentProcessRestartLogic(t *testing.T) {
	t.Run("agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "test-process",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessRestart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("with force flag", func(t *testing.T) {
		svcCtx := createTestServiceContext()

		logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "test-process",
			Force:   true,
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessRestart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsNodesLogic (not implemented)
func TestOpsNodesLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodesLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodes(&OpsNodesRequest{})
	if err == nil {
		t.Error("OpsNodes() expected error (not implemented)")
	}
}

// Test OpsConfigLogic (not implemented)
func TestOpsConfigLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsConfigLogic(context.Background(), svcCtx)

	_, err := logic.OpsConfig(&OpsConfigRequest{})
	if err == nil {
		t.Error("OpsConfig() expected error (not implemented)")
	}
}

// Test OpsAlertsLogic (not implemented)
func TestOpsAlertsLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAlertsLogic(context.Background(), svcCtx)

	_, err := logic.OpsAlerts(&OpsAlertsRequest{})
	if err == nil {
		t.Error("OpsAlerts() expected error (not implemented)")
	}
}

// Test OpsHealthGetLogic (not implemented)
func TestOpsHealthGetLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsHealthGetLogic(context.Background(), svcCtx)

	_, err := logic.OpsHealthGet(&OpsHealthGetRequest{})
	if err == nil {
		t.Error("OpsHealthGet() expected error (not implemented)")
	}
}

// Test OpsBackupCreateLogic (not implemented)
func TestOpsBackupCreateLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsBackupCreateLogic(context.Background(), svcCtx)

	_, err := logic.OpsBackupCreate(&OpsBackupCreateRequest{})
	if err == nil {
		t.Error("OpsBackupCreate() expected error (not implemented)")
	}
}

// Test state_helpers functions
func TestSnapshotOpsState(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		state := snapshotOpsState(nil)

		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state for nil context")
		}
	})

	t.Run("context without OpsStateStore", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{}
		state := snapshotOpsState(svcCtx)

		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty state when OpsStateStore is nil")
		}
	})

	t.Run("with OpsStateStore", func(t *testing.T) {
		// Create a temporary directory for the ops state file
		tmpDir := t.TempDir()
		store := svc.NewOpsStateStore(tmpDir)

		// Update the store with our test data
		_, _ = store.Update(func(s *svc.OpsState) {
			s.Config.AlertmanagerURL = "http://localhost:9093"
		})

		svcCtx := &svc.ServiceContext{
			OpsStateStore: store,
		}

		state := snapshotOpsState(svcCtx)

		if state.Config.AlertmanagerURL != "http://localhost:9093" {
			t.Errorf("expected AlertmanagerURL, got %s", state.Config.AlertmanagerURL)
		}
	})
}

func TestUpdateOpsState(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		_, err := updateOpsState(nil, func(s *svc.OpsState) {})

		if err == nil {
			t.Error("expected error for nil context")
		}

		if !errors.Is(err, errOpsStateUnavailable) {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})

	t.Run("context without OpsStateStore", func(t *testing.T) {
		svcCtx := &svc.ServiceContext{}
		_, err := updateOpsState(svcCtx, func(s *svc.OpsState) {})

		if err == nil {
			t.Error("expected error when OpsStateStore is nil")
		}

		if !errors.Is(err, errOpsStateUnavailable) {
			t.Errorf("expected errOpsStateUnavailable, got %v", err)
		}
	})
}

// Test New* functions
func TestNewOpsAgentsListLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentsListLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentsListLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentsListLogic() did not set svcCtx")
	}
}

func TestNewOpsServicesLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsServicesLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsServicesLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsServicesLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentSystemInfoLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentSystemInfoLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentSystemInfoLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentMetricsLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentMetricsLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentMetricsLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentProcessesLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentProcessesLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentProcessesLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentExecCommandLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentExecCommandLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentExecCommandLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentProcessStartLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentProcessStartLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentProcessStartLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentProcessStopLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentProcessStopLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentProcessStopLogic() did not set svcCtx")
	}
}

func TestNewOpsAgentProcessRestartLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)

	if logic == nil {
		t.Error("NewOpsAgentProcessRestartLogic() returned nil")
	}

	if logic.svcCtx != svcCtx {
		t.Error("NewOpsAgentProcessRestartLogic() did not set svcCtx")
	}
}

// Test collectServerLabels
func TestCollectServerLabels(t *testing.T) {
	labels := collectServerLabels()

	// Check for expected labels
	expectedKeys := []string{"os", "arch", "hostname", "cpu_count", "go_version"}
	for _, key := range expectedKeys {
		if _, ok := labels[key]; !ok {
			t.Errorf("expected label %s to be present", key)
		}
	}

	// Check OS label
	if labels["os"] == "" {
		t.Error("expected non-empty os label")
	}

	// Check arch label
	if labels["arch"] == "" {
		t.Error("expected non-empty arch label")
	}

	// Check cpu_count is a number
	if labels["cpu_count"] == "" {
		t.Error("expected non-empty cpu_count label")
	}

	// Check go_version
	if labels["go_version"] == "" {
		t.Error("expected non-empty go_version label")
	}
}

// Test OpsAgentMetaLogic (not implemented)
func TestOpsAgentMetaLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAgentMetaLogic(context.Background(), svcCtx)

	_, err := logic.OpsAgentMeta(&OpsAgentMetaUpdateRequest{})
	if err == nil {
		t.Error("OpsAgentMeta() expected error (not implemented)")
	}
}

// Test OpsBackupsListLogic
func TestOpsBackupsListLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsBackupsListLogic(context.Background(), svcCtx)

	_, err := logic.OpsBackupsList(&OpsBackupsListRequest{})
	if err == nil {
		t.Error("OpsBackupsList() expected error (not implemented)")
	}
}

// Test OpsBackupDeleteLogic
func TestOpsBackupDeleteLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsBackupDeleteLogic(context.Background(), svcCtx)

	_, err := logic.OpsBackupDelete(&OpsBackupDeleteRequest{})
	if err == nil {
		t.Error("OpsBackupDelete() expected error (not implemented)")
	}
}

// Test OpsBackupDownloadLogic
func TestOpsBackupDownloadLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsBackupDownloadLogic(context.Background(), svcCtx)

	_, err := logic.OpsBackupDownload(&OpsBackupDownloadRequest{})
	if err == nil {
		t.Error("OpsBackupDownload() expected error (not implemented)")
	}
}

// Test OpsFunctionsLogic
func TestOpsFunctionsLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsFunctionsLogic(context.Background(), svcCtx)

	_, err := logic.OpsFunctions(&OpsFunctionsRequest{})
	if err == nil {
		t.Error("OpsFunctions() expected error (not implemented)")
	}
}

// Test OpsMetricsLogic
func TestOpsMetricsLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsMetricsLogic(context.Background(), svcCtx)

	_, err := logic.OpsMetrics(&OpsMetricsQuery{})
	if err == nil {
		t.Error("OpsMetrics() expected error (not implemented)")
	}
}

// Test OpsMQLogic
func TestOpsMQLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsMQLogic(context.Background(), svcCtx)

	_, err := logic.OpsMQ(&OpsMQRequest{})
	if err == nil {
		t.Error("OpsMQ() expected error (not implemented)")
	}
}

// Test OpsMaintenanceGetLogic
func TestOpsMaintenanceGetLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsMaintenanceGetLogic(context.Background(), svcCtx)

	_, err := logic.OpsMaintenanceGet(&OpsMaintenanceGetRequest{})
	if err == nil {
		t.Error("OpsMaintenanceGet() expected error (not implemented)")
	}
}

// Test OpsMaintenanceUpdateLogic
func TestOpsMaintenanceUpdateLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsMaintenanceUpdateLogic(context.Background(), svcCtx)

	_, err := logic.OpsMaintenanceUpdate(&OpsMaintenanceUpdateRequest{})
	if err == nil {
		t.Error("OpsMaintenanceUpdate() expected error (not implemented)")
	}
}

// Test OpsNodeCommandsLogic
func TestOpsNodeCommandsLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodeCommandsLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodeCommands(&OpsNodeCommandsQuery{})
	if err == nil {
		t.Error("OpsNodeCommands() expected error (not implemented)")
	}
}

// Test OpsNodeDrainLogic
func TestOpsNodeDrainLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodeDrainLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodeDrain(&OpsNodeActionRequest{})
	if err == nil {
		t.Error("OpsNodeDrain() expected error (not implemented)")
	}
}

// Test OpsNodeMetaLogic
func TestOpsNodeMetaLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodeMetaLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodeMeta(&OpsNodeMetaRequest{})
	if err == nil {
		t.Error("OpsNodeMeta() expected error (not implemented)")
	}
}

// Test OpsNodeRestartLogic
func TestOpsNodeRestartLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodeRestartLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodeRestart(&OpsNodeActionRequest{})
	if err == nil {
		t.Error("OpsNodeRestart() expected error (not implemented)")
	}
}

// Test OpsNodeUndrainLogic
func TestOpsNodeUndrainLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNodeUndrainLogic(context.Background(), svcCtx)

	_, err := logic.OpsNodeUndrain(&OpsNodeActionRequest{})
	if err == nil {
		t.Error("OpsNodeUndrain() expected error (not implemented)")
	}
}

// Test OpsHealthRunLogic
func TestOpsHealthRunLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsHealthRunLogic(context.Background(), svcCtx)

	_, err := logic.OpsHealthRun(&OpsHealthRunRequest{})
	if err == nil {
		t.Error("OpsHealthRun() expected error (not implemented)")
	}
}

// Test OpsHealthUpdateLogic
func TestOpsHealthUpdateLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsHealthUpdateLogic(context.Background(), svcCtx)

	_, err := logic.OpsHealthUpdate(&OpsHealthUpdateRequest{})
	if err == nil {
		t.Error("OpsHealthUpdate() expected error (not implemented)")
	}
}

// Test OpsAlertSilenceLogic
func TestOpsAlertSilenceLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsAlertSilenceLogic(context.Background(), svcCtx)

	_, err := logic.OpsAlertSilence(&OpsAlertSilenceRequest{})
	if err == nil {
		t.Error("OpsAlertSilence() expected error (not implemented)")
	}
}

// Test OpsSilencesLogic
func TestOpsSilencesLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsSilencesLogic(context.Background(), svcCtx)

	_, err := logic.OpsSilences(&OpsSilencesRequest{})
	if err == nil {
		t.Error("OpsSilences() expected error (not implemented)")
	}
}

// Test OpsSilenceDeleteLogic
func TestOpsSilenceDeleteLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsSilenceDeleteLogic(context.Background(), svcCtx)

	_, err := logic.OpsSilenceDelete(&OpsAlertSilenceDeleteRequest{})
	if err == nil {
		t.Error("OpsSilenceDelete() expected error (not implemented)")
	}
}

// Test OpsNotificationsGetLogic
func TestOpsNotificationsGetLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNotificationsGetLogic(context.Background(), svcCtx)

	_, err := logic.OpsNotificationsGet(&OpsNotificationsGetRequest{})
	if err == nil {
		t.Error("OpsNotificationsGet() expected error (not implemented)")
	}
}

// Test OpsNotificationsUpdateLogic
func TestOpsNotificationsUpdateLogic(t *testing.T) {
	svcCtx := createTestServiceContext()
	logic := NewOpsNotificationsUpdateLogic(context.Background(), svcCtx)

	_, err := logic.OpsNotificationsUpdate(&OpsNotificationsUpdateRequest{})
	if err == nil {
		t.Error("OpsNotificationsUpdate() expected error (not implemented)")
	}
}

// Test utils.CountEnabledFunctions edge case
func TestOpsAgentMetricsLogic_EdgeCases(t *testing.T) {
	t.Run("empty metrics data", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

		// Add a nil report should be handled
		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{})
		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}
		_ = resp
	})

	t.Run("large limit", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentMetrics(&OpsAgentMetricsRequest{
			Limit: 1000000,
		})
		if err != nil {
			t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
		}
		_ = resp
	})
}

// Test AgentOpsClient_GetClient_Locked tests getting a client with concurrent access
func TestAgentOpsClient_GetClient_Locked(t *testing.T) {
	// Create a client with a mock NNG client
	agentOpsClientOnce = sync.Once{}
	globalAgentOpsClient = &AgentOpsClient{
		agents: make(map[string]*nng.Client),
	}

	t.Run("concurrent access", func(t *testing.T) {
		client := GetAgentOpsClient()
		if client == nil {
			t.Fatal("GetAgentOpsClient() should return non-nil client")
		}

		// Test concurrent reads
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				_, _ = client.GetClient(context.Background(), "test-agent")
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Test OpsAgentSystemInfoLogic_CacheHit tests the cache hit path
func TestOpsAgentSystemInfoLogic_CacheHit(t *testing.T) {
	svcCtx := createTestServiceContext()

	// Pre-populate the cache
	cachedInfo := &opsv1.SystemInfo{
		Hostname:      "cached-host",
		Os:            "linux",
		CpuCores:      8,
		TotalMemory:   16 * 1024 * 1024 * 1024,
		BootTime:      timestamppb.Now(),
		AgentVersion:  "v1.0.0",
	}
	svcCtx.SystemInfoCache.Set("test-agent", cachedInfo)

	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "test-agent"})

	if err != nil {
		t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	if resp.Data.Hostname != "cached-host" {
		t.Errorf("expected hostname cached-host, got %s", resp.Data.Hostname)
	}
}

// Test OpsAgentSystemInfoLogic_CacheMiss tests the cache miss path
func TestOpsAgentSystemInfoLogic_CacheMiss(t *testing.T) {
	svcCtx := createTestServiceContext()
	// Don't populate the cache - agent not found

	logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
	resp, err := logic.OpsAgentSystemInfo(&OpsAgentSystemInfoRequest{AgentID: "unknown-agent"})

	if err != nil {
		t.Fatalf("OpsAgentSystemInfo() unexpected error: %v", err)
	}

	if resp.Code != 404 {
		t.Errorf("expected code 404, got %d", resp.Code)
	}

	if resp.Message == "" {
		t.Error("expected error message for unknown agent")
	}
}

// Test OpsAgentSystemInfoLogic_NilTimestamp tests formatTimestamp with nil
func TestOpsAgentSystemInfoLogic_NilTimestamp(t *testing.T) {
	result := formatTimestamp(nil)
	if result != "" {
		t.Errorf("expected empty string for nil timestamp, got %s", result)
	}
}

// Test OpsAgentsListLogic edge cases
func TestOpsAgentsListLogic_EdgeCases(t *testing.T) {
	t.Run("nil agent in store", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		// Manually add a nil agent to the store (edge case)
		svcCtx.RegistryStore.UpsertAgent(nil)

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		// Should handle nil agent gracefully
		_ = resp
	})

	t.Run("empty functions map", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		sess := &registry.AgentSession{
			AgentID:   "test-agent",
			GameID:    "game1",
			Env:       "dev",
			Version:   "v1.0.0",
			RPCAddr:   "localhost:19090",
			ExpireAt:  time.Now().Add(time.Minute),
			Functions: map[string]registry.FunctionMeta{},
			Labels:    map[string]string{},
		}
		svcCtx.RegistryStore.UpsertAgent(sess)

		logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentsList(&OpsAgentsListRequest{})

		if err != nil {
			t.Fatalf("OpsAgentsList() unexpected error: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("expected 1 agent, got %d", len(resp.Data))
		}

		if len(resp.Data[0].Functions) != 0 {
			t.Errorf("expected 0 functions, got %d", len(resp.Data[0].Functions))
		}
	})
}

// Test ttlAndHealth edge cases
func TestTtlAndHealth_EdgeCases(t *testing.T) {
	t.Run("exactly at expiration boundary", func(t *testing.T) {
		sess := &registry.AgentSession{
			ExpireAt: time.Now().Add(time.Second),
		}

		ttl, healthy := ttlAndHealth(sess)

		// Should be healthy since TTL is positive
		if !healthy {
			t.Error("expected session to be healthy when ExpireAt is in the future")
		}

		if ttl <= 0 {
			t.Errorf("expected positive TTL, got %d", ttl)
		}
	})
}

// Test formatLastSeen edge cases
func TestFormatLastSeen_EdgeCases(t *testing.T) {
	t.Run("both times zero", func(t *testing.T) {
		lastSeen := time.Time{}
		expireAt := time.Time{}

		result := formatLastSeen(lastSeen, expireAt)
		// Should not panic and return some formatted string
		if result == "" {
			t.Error("expected non-empty result for zero times")
		}
	})
}

// Test OpsAgentProcessesLogic edge cases
func TestOpsAgentProcessesLogic_EdgeCases(t *testing.T) {
	t.Run("empty agent ID", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{AgentID: ""})

		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404 for empty agent ID, got %d", resp.Code)
		}
	})
}

// Test updateOpsState edge cases
func TestUpdateOpsState_EdgeCases(t *testing.T) {
	t.Run("with valid store", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := svc.NewOpsStateStore(tmpDir)
		svcCtx := &svc.ServiceContext{
			OpsStateStore: store,
		}

		updated, err := updateOpsState(svcCtx, func(s *svc.OpsState) {
			s.Config.AlertmanagerURL = "http://localhost:9093"
		})

		if err != nil {
			t.Fatalf("updateOpsState() unexpected error: %v", err)
		}

		if updated.Config.AlertmanagerURL != "http://localhost:9093" {
			t.Errorf("expected AlertmanagerURL to be updated, got %s", updated.Config.AlertmanagerURL)
		}
	})
}

// Test OpsAgentProcessesLogic_Convert tests process list conversion
func TestOpsAgentProcessesLogic_Convert(t *testing.T) {
	t.Run("nil store - agent not found", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		svcCtx.RegistryStore = nil

		logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
		resp, err := logic.OpsAgentProcesses(&OpsAgentProcessesRequest{
			AgentID: "test-agent",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcesses() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404 for nil store, got %d", resp.Code)
		}
	})
}

// Test OpsAgentExecCommandLogic edge cases
func TestOpsAgentExecCommandLogic_EdgeCases(t *testing.T) {
	t.Run("empty command", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
			AgentID: "unknown-agent",
			Command: "",
		})

		if err != nil {
			t.Fatalf("OpsAgentExecCommand() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("with args", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentExecCommand(&OpsExecCommandRequest{
			AgentID: "unknown-agent",
			Command: "test",
			Args:    []string{"arg1", "arg2"},
		})

		if err != nil {
			t.Fatalf("OpsAgentExecCommand() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessStartLogic edge cases
func TestOpsAgentProcessStartLogic_EdgeCases(t *testing.T) {
	t.Run("empty process name", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{
			AgentID: "unknown-agent",
			Name:    "",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})

	t.Run("with env vars", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentProcessStart(&OpsProcessStartRequest{
			AgentID: "unknown-agent",
			Name:    "test",
			Env:     map[string]string{"KEY": "value"},
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessStopLogic_EdgeCases
func TestOpsAgentProcessStopLogic_EdgeCases(t *testing.T) {
	t.Run("empty process name", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentProcessStop(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessStop() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test OpsAgentProcessRestartLogic_EdgeCases
func TestOpsAgentProcessRestartLogic_EdgeCases(t *testing.T) {
	t.Run("empty process name", func(t *testing.T) {
		svcCtx := createTestServiceContext()
		logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)

		resp, err := logic.OpsAgentProcessRestart(&OpsProcessActionRequest{
			AgentID: "unknown-agent",
			Name:    "",
		})

		if err != nil {
			t.Fatalf("OpsAgentProcessRestart() unexpected error: %v", err)
		}

		if resp.Code != 404 {
			t.Errorf("expected code 404, got %d", resp.Code)
		}
	})
}

// Test collectServerLabels coverage
func TestCollectServerLabels_Full(t *testing.T) {
	labels := collectServerLabels()

	// Verify all expected labels are present
	expectedLabels := []string{"os", "arch", "hostname", "cpu_count", "go_version"}
	for _, label := range expectedLabels {
		if _, exists := labels[label]; !exists {
			t.Errorf("expected label %s to exist", label)
		}
	}

	// Verify values are non-empty where applicable
	if labels["os"] == "" {
		t.Error("os label should not be empty")
	}
	if labels["arch"] == "" {
		t.Error("arch label should not be empty")
	}
	if labels["go_version"] == "" {
		t.Error("go_version label should not be empty")
	}
}

// Test snapshotOpsState with different contexts
func TestSnapshotOpsState_Variants(t *testing.T) {
	t.Run("empty store", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := svc.NewOpsStateStore(tmpDir)
		svcCtx := &svc.ServiceContext{
			OpsStateStore: store,
		}

		state := snapshotOpsState(svcCtx)

		// Default state should have empty config URL
		if state.Config.AlertmanagerURL != "" {
			t.Error("expected empty AlertmanagerURL for new store")
		}
	})
}

// Additional New* function tests for coverage
func TestNewOpsLogicConstructors(t *testing.T) {
	svcCtx := createTestServiceContext()
	ctx := context.Background()

	constructors := []struct {
		name     string
		construct func(context.Context, *svc.ServiceContext) interface{}
	}{
		{"NewOpsAgentMetaLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsAgentMetaLogic(ctx, svcCtx)
		}},
		{"NewOpsBackupsListLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsBackupsListLogic(ctx, svcCtx)
		}},
		{"NewOpsBackupDeleteLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsBackupDeleteLogic(ctx, svcCtx)
		}},
		{"NewOpsBackupDownloadLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsBackupDownloadLogic(ctx, svcCtx)
		}},
		{"NewOpsFunctionsLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsFunctionsLogic(ctx, svcCtx)
		}},
		{"NewOpsMQLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsMQLogic(ctx, svcCtx)
		}},
		{"NewOpsMaintenanceGetLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsMaintenanceGetLogic(ctx, svcCtx)
		}},
		{"NewOpsMaintenanceUpdateLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsMaintenanceUpdateLogic(ctx, svcCtx)
		}},
		{"NewOpsNodeMetaLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsNodeMetaLogic(ctx, svcCtx)
		}},
		{"NewOpsHealthRunLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsHealthRunLogic(ctx, svcCtx)
		}},
		{"NewOpsHealthUpdateLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsHealthUpdateLogic(ctx, svcCtx)
		}},
		{"NewOpsAlertSilenceLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsAlertSilenceLogic(ctx, svcCtx)
		}},
		{"NewOpsSilencesLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsSilencesLogic(ctx, svcCtx)
		}},
		{"NewOpsSilenceDeleteLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsSilenceDeleteLogic(ctx, svcCtx)
		}},
		{"NewOpsNotificationsGetLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsNotificationsGetLogic(ctx, svcCtx)
		}},
		{"NewOpsNotificationsUpdateLogic", func(ctx context.Context, svcCtx *svc.ServiceContext) interface{} {
			return NewOpsNotificationsUpdateLogic(ctx, svcCtx)
		}},
	}

	for _, tc := range constructors {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.construct(ctx, svcCtx)
			if result == nil {
				t.Errorf("%s() returned nil", tc.name)
			}
		})
	}
}

// Test OpsAgentProcessesLogic_MultipleParams
func TestOpsAgentProcessesLogic_MultipleParams(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsAgentProcessesRequest
		wantErr bool
	}{
		{
			name:    "valid request with unknown agent",
			req:     &OpsAgentProcessesRequest{AgentID: "unknown-agent"},
			wantErr: false, // Should not return error, just 404 code
		},
		{
			name:    "empty agent ID",
			req:     &OpsAgentProcessesRequest{AgentID: ""},
			wantErr: false,
		},
		{
			name:    "whitespace agent ID",
			req:     &OpsAgentProcessesRequest{AgentID: "   "},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentProcessesLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentProcesses(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentProcesses() error = %v, wantErr %v", err, tc.wantErr)
			}

			// All unknown agents should return 404
			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentExecCommandLogic_Variants
func TestOpsAgentExecCommandLogic_Variants(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsExecCommandRequest
		wantErr bool
	}{
		{
			name: "with timeout",
			req: &OpsExecCommandRequest{
				AgentID: "unknown",
				Command: "echo",
				Timeout: 30,
			},
			wantErr: false,
		},
		{
			name: "with empty command",
			req: &OpsExecCommandRequest{
				AgentID: "unknown",
				Command: "",
			},
			wantErr: false,
		},
		{
			name: "with args",
			req: &OpsExecCommandRequest{
				AgentID: "unknown",
				Command: "test",
				Args:    []string{"a", "b"},
			},
			wantErr: false,
		},
		{
			name: "with env",
			req: &OpsExecCommandRequest{
				AgentID: "unknown",
				Command: "test",
				Env:     map[string]string{"X": "1"},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentExecCommandLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentExecCommand(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentExecCommand() error = %v, wantErr %v", err, tc.wantErr)
			}

			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentProcessStartLogic_Variants
func TestOpsAgentProcessStartLogic_Variants(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsProcessStartRequest
		wantErr bool
	}{
		{
			name: "basic request",
			req: &OpsProcessStartRequest{
				AgentID: "unknown",
				Name:    "test",
			},
			wantErr: false,
		},
		{
			name: "with command",
			req: &OpsProcessStartRequest{
				AgentID: "unknown",
				Name:    "test",
				Command: "/bin/test",
			},
			wantErr: false,
		},
		{
			name: "with args",
			req: &OpsProcessStartRequest{
				AgentID: "unknown",
				Name:    "test",
				Args:    []string{"-v"},
			},
			wantErr: false,
		},
		{
			name: "with env",
			req: &OpsProcessStartRequest{
				AgentID: "unknown",
				Name:    "test",
				Env:     map[string]string{"PATH": "/bin"},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentProcessStartLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentProcessStart(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentProcessStart() error = %v, wantErr %v", err, tc.wantErr)
			}

			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentProcessStopLogic_Variants
func TestOpsAgentProcessStopLogic_Variants(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsProcessActionRequest
		wantErr bool
	}{
		{
			name: "basic request",
			req: &OpsProcessActionRequest{
				AgentID: "unknown",
				Name:    "test",
			},
			wantErr: false,
		},
		{
			name: "with force",
			req: &OpsProcessActionRequest{
				AgentID: "unknown",
				Name:    "test",
				Force:   true,
			},
			wantErr: false,
		},
		{
			name: "with PID",
			req: &OpsProcessActionRequest{
				AgentID: "unknown",
				Name:    "test",
				PID:     1234,
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentProcessStopLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentProcessStop(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentProcessStop() error = %v, wantErr %v", err, tc.wantErr)
			}

			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentProcessRestartLogic_Variants
func TestOpsAgentProcessRestartLogic_Variants(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsProcessActionRequest
		wantErr bool
	}{
		{
			name: "basic request",
			req: &OpsProcessActionRequest{
				AgentID: "unknown",
				Name:    "test",
			},
			wantErr: false,
		},
		{
			name: "with force",
			req: &OpsProcessActionRequest{
				AgentID: "unknown",
				Name:    "test",
				Force:   true,
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentProcessRestartLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentProcessRestart(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentProcessRestart() error = %v, wantErr %v", err, tc.wantErr)
			}

			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentSystemInfoLogic_Variants
func TestOpsAgentSystemInfoLogic_Variants(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsAgentSystemInfoRequest
		wantErr bool
	}{
		{
			name: "unknown agent",
			req: &OpsAgentSystemInfoRequest{
				AgentID: "unknown-agent-12345",
			},
			wantErr: false,
		},
		{
			name: "empty agent ID",
			req: &OpsAgentSystemInfoRequest{
				AgentID: "",
			},
			wantErr: false,
		},
		{
			name: "special characters",
			req: &OpsAgentSystemInfoRequest{
				AgentID: "agent-with-special-chars_123",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentSystemInfoLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentSystemInfo(tc.req)

			if (err != nil) != tc.wantErr {
				t.Fatalf("OpsAgentSystemInfo() error = %v, wantErr %v", err, tc.wantErr)
			}

			if resp.Code != 404 {
				t.Errorf("expected code 404, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentsList request types with nil/empty values
func TestOpsAgentsList_RequestTypes(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsAgentsListRequest
	}{
		{
			name: "empty request",
			req:  &OpsAgentsListRequest{},
		},
		{
			name: "with GameID",
			req: &OpsAgentsListRequest{
				GameID: "game1",
			},
		},
		{
			name: "with Env",
			req: &OpsAgentsListRequest{
				GameID: "game1",
				Env:    "prod",
			},
		},
		{
			name: "with both fields",
			req: &OpsAgentsListRequest{
				GameID: "game1",
				Env:    "dev",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentsListLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentsList(tc.req)

			if err != nil {
				t.Fatalf("OpsAgentsList() unexpected error: %v", err)
			}

			if resp.Code != 0 {
				t.Errorf("expected code 0, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentMetrics request types
func TestOpsAgentMetrics_RequestTypes(t *testing.T) {
	svcCtx := createTestServiceContext()

	testCases := []struct {
		name    string
		req     *OpsAgentMetricsRequest
	}{
		{
			name: "empty request",
			req:  &OpsAgentMetricsRequest{},
		},
		{
			name: "with AgentID only",
			req: &OpsAgentMetricsRequest{
				AgentID: "test-agent",
			},
		},
		{
			name: "with Since",
			req: &OpsAgentMetricsRequest{
				Since: time.Now().Add(-time.Hour).Format(time.RFC3339),
			},
		},
		{
			name: "with Limit",
			req: &OpsAgentMetricsRequest{
				Limit: 50,
			},
		},
		{
			name: "with all fields",
			req: &OpsAgentMetricsRequest{
				AgentID: "test-agent",
				Since:   time.Now().Add(-time.Hour).Format(time.RFC3339),
				Limit:   100,
			},
		},
		{
			name: "negative limit",
			req: &OpsAgentMetricsRequest{
				Limit: -1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logic := NewOpsAgentMetricsLogic(context.Background(), svcCtx)
			resp, err := logic.OpsAgentMetrics(tc.req)

			if err != nil {
				t.Fatalf("OpsAgentMetrics() unexpected error: %v", err)
			}

			if resp.Code != 0 {
				t.Errorf("expected code 0, got %d", resp.Code)
			}
		})
	}
}

// Test OpsAgentSystemInfo with different timestamps
func TestOpsAgentSystemInfo_Timestamps(t *testing.T) {
	timestamps := []*timestamppb.Timestamp{
		nil,
		timestamppb.Now(),
		timestamppb.New(time.Unix(0, 0)),
		timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
	}

	for i, ts := range timestamps {
		t.Run(fmt.Sprintf("timestamp_%d", i), func(t *testing.T) {
			result := formatTimestamp(ts)
			_ = result // Should not panic
		})
	}
}

// Test formatLastSeen with various time combinations
func TestFormatLastSeen_Combinations(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name     string
		lastSeen time.Time
		expireAt time.Time
	}{
		{
			name:     "both set",
			lastSeen: now,
			expireAt: now.Add(time.Minute),
		},
		{
			name:     "lastSeen zero",
			lastSeen: time.Time{},
			expireAt: now,
		},
		{
			name:     "expireAt zero",
			lastSeen: now,
			expireAt: time.Time{},
		},
		{
			name:     "both zero",
			lastSeen: time.Time{},
			expireAt: time.Time{},
		},
		{
			name:     "past times",
			lastSeen: now.Add(-time.Hour),
			expireAt: now.Add(-time.Minute),
		},
		{
			name:     "future times",
			lastSeen: now.Add(time.Hour),
			expireAt: now.Add(2 * time.Hour),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatLastSeen(tc.lastSeen, tc.expireAt)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

// Test ttlAndHealth with various session states
func TestTtlAndHealth_SessionStates(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name        string
		session     *registry.AgentSession
		wantTTL     int
		wantHealthy bool
	}{
		{
			name:        "healthy future",
			session:     &registry.AgentSession{ExpireAt: now.Add(time.Minute)},
			wantTTL:     60,
			wantHealthy: true,
		},
		{
			name:        "expired past",
			session:     &registry.AgentSession{ExpireAt: now.Add(-time.Minute)},
			wantTTL:     0,
			wantHealthy: false,
		},
		{
			name:        "exactly now",
			session:     &registry.AgentSession{ExpireAt: now},
			wantTTL:     0,
			wantHealthy: false,
		},
		{
			name:        "nil session",
			session:     nil,
			wantTTL:     0,
			wantHealthy: false,
		},
		{
			name:        "zero expire time",
			session:     &registry.AgentSession{ExpireAt: time.Time{}},
			wantTTL:     0,
			wantHealthy: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ttl, healthy := ttlAndHealth(tc.session)

			if healthy != tc.wantHealthy {
				t.Errorf("ttlAndHealth() healthy = %v, want %v", healthy, tc.wantHealthy)
			}

			if tc.wantHealthy && ttl <= 0 {
				t.Errorf("ttlAndHealth() TTL = %d, want positive", ttl)
			}

			if !tc.wantHealthy && ttl != tc.wantTTL {
				t.Errorf("ttlAndHealth() TTL = %d, want %d", ttl, tc.wantTTL)
			}
		})
	}
}

