package agent

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
)

func newTestServiceContext() *svc.ServiceContext {
	cfg := config.Config{}
	cfg.Agent.ID = "test-agent-001"
	cfg.Agent.GameID = "test-game"
	cfg.Agent.Env = "development"
	cfg.Agent.LocalAddr = "localhost:19090"
	cfg.Server.Addr = "localhost:8443"
	cfg.Upstream.HeartbeatInterval = 30

	return &svc.ServiceContext{
		Config:        cfg,
		StartTime:     time.Now().Add(-time.Hour), // Started 1 hour ago
		Core:          nil,                        // No core for unit tests
		LocalGRPCAddr: "localhost:19090",
	}
}

func TestAgentHealth_ValidRequest(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentHealth(req)
	if err != nil {
		t.Errorf("AgentHealth() error = %v", err)
	}
	if resp == nil {
		t.Fatal("AgentHealth() returned nil response")
	}
	if resp.Status != "stopped" { // Core is nil
		t.Errorf("AgentHealth() Status = %v, want 'stopped'", resp.Status)
	}
	if resp.Uptime <= 0 {
		t.Errorf("AgentHealth() Uptime = %v, want > 0", resp.Uptime)
	}
	// Uptime should be approximately 1 hour (3600 seconds)
	if resp.Uptime < 3500 || resp.Uptime > 3700 {
		t.Errorf("AgentHealth() Uptime = %v, want ~3600", resp.Uptime)
	}
}

func TestAgentHealth_EmptyAgentId(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "",
	}

	_, err := logic.AgentHealth(req)
	if err == nil {
		t.Error("AgentHealth() with empty agent_id should return error")
	}
}

func TestAgentHealth_NilRequest(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	_, err := logic.AgentHealth(nil)
	if err == nil {
		t.Error("AgentHealth() with nil request should return error")
	}
}

func TestAgentHealth_AgentIdMismatch(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "wrong-agent-id",
	}

	_, err := logic.AgentHealth(req)
	if err == nil {
		t.Error("AgentHealth() with mismatched agent_id should return error")
	}
}

func TestAgentHealth_WhitespaceAgentId(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "   ",
	}

	_, err := logic.AgentHealth(req)
	if err == nil {
		t.Error("AgentHealth() with whitespace-only agent_id should return error")
	}
}

func TestAgentHealth_MemoryStats(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentHealth(req)
	if err != nil {
		t.Errorf("AgentHealth() error = %v", err)
	}
	if resp.Memory <= 0 {
		t.Errorf("AgentHealth() Memory = %v, want > 0", resp.Memory)
	}
}

func TestAgentHealth_CPUUsage(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)

	req := &types.AgentHealthRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentHealth(req)
	if err != nil {
		t.Errorf("AgentHealth() error = %v", err)
	}
	if resp.Cpu < 0 || resp.Cpu > 100 {
		t.Errorf("AgentHealth() Cpu = %v, want 0-100", resp.Cpu)
	}
}

func TestAgentHealth_Concurrent(t *testing.T) {
	svcCtx := newTestServiceContext()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logic := NewAgentHealthLogic(nil, svcCtx)
			req := &types.AgentHealthRequest{
				AgentId: "test-agent-001",
			}
			resp, err := logic.AgentHealth(req)
			if err != nil {
				t.Errorf("Concurrent AgentHealth() error = %v", err)
			}
			if resp == nil {
				t.Error("Concurrent AgentHealth() returned nil response")
			}
		}()
	}

	wg.Wait()
}

func TestCalculateCPUUsage_Caching(t *testing.T) {
	// Reset the tracker for this test
	cpuUsageTracker.lastCPUTime = time.Time{}
	cpuUsageTracker.lastGCPauseNs = 0
	cpuUsageTracker.lastCPUUsage = 0

	// Use actual runtime.MemStats
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// First call should calculate
	cpu1 := calculateCPUUsage(&mem)

	// CPU usage should be within bounds
	if cpu1 < 0 || cpu1 > 100 {
		t.Errorf("calculateCPUUsage() = %v, want 0-100", cpu1)
	}

	// Rapid second call should return cached value
	cpu2 := calculateCPUUsage(&mem)

	// Both should be valid
	if cpu2 < 0 || cpu2 > 100 {
		t.Errorf("calculateCPUUsage() cached = %v, want 0-100", cpu2)
	}
}

// Table-driven tests for validation
func TestAgentHealth_Validation(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		wantErr bool
	}{
		{"valid agent id", "test-agent-001", false},
		{"empty agent id", "", true},
		{"whitespace only", "   ", true},
		{"wrong agent id", "wrong-agent", true},
		{"agent id with spaces", "  test-agent-001  ", false}, // Should trim
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcCtx := newTestServiceContext()
			logic := NewAgentHealthLogic(nil, svcCtx)

			req := &types.AgentHealthRequest{
				AgentId: tt.agentID,
			}

			_, err := logic.AgentHealth(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AgentHealth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkAgentHealth(b *testing.B) {
	svcCtx := newTestServiceContext()
	logic := NewAgentHealthLogic(nil, svcCtx)
	req := &types.AgentHealthRequest{
		AgentId: "test-agent-001",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = logic.AgentHealth(req)
	}
}
