package agent

import (
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
)

func TestAgentMetrics_ValidRequest(t *testing.T) {
	svcCtx := newTestServiceContext()
	// Set some timestamps
	now := time.Now()
	svcCtx.SetRegisteredAt(now.Add(-time.Hour))
	svcCtx.SetLastHeartbeat(now.Add(-time.Minute))

	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}
	if resp == nil {
		t.Fatal("AgentMetrics() returned nil response")
	}
	if resp.Metrics == nil {
		t.Fatal("AgentMetrics() returned nil metrics")
	}

	// Check expected fields exist
	expectedFields := []string{
		"agentId",
		"uptime_sec",
		"functions",
		"instances",
		"active_jobs",
		"grpc_addr",
		"upstream_addr",
		"heartbeat_sec",
		"status",
		"registered_at",
		"last_heartbeat",
	}

	for _, field := range expectedFields {
		if _, ok := resp.Metrics[field]; !ok {
			t.Errorf("AgentMetrics() missing field: %s", field)
		}
	}
}

func TestAgentMetrics_EmptyAgentId(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "",
	}

	_, err := logic.AgentMetrics(req)
	if err == nil {
		t.Error("AgentMetrics() with empty agent_id should return error")
	}
}

func TestAgentMetrics_NilRequest(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	_, err := logic.AgentMetrics(nil)
	if err == nil {
		t.Error("AgentMetrics() with nil request should return error")
	}
}

func TestAgentMetrics_AgentIdMismatch(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "wrong-agent-id",
	}

	_, err := logic.AgentMetrics(req)
	if err == nil {
		t.Error("AgentMetrics() with mismatched agent_id should return error")
	}
}

func TestAgentMetrics_StatusStopped(t *testing.T) {
	svcCtx := newTestServiceContext()
	svcCtx.Core = nil // No core means stopped

	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	status, ok := resp.Metrics["status"]
	if !ok {
		t.Fatal("AgentMetrics() missing status field")
	}
	if status != "stopped" {
		t.Errorf("AgentMetrics() status = %v, want 'stopped'", status)
	}
}

func TestAgentMetrics_UptimeValue(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	uptimeSec, ok := resp.Metrics["uptime_sec"].(int64)
	if !ok {
		t.Fatal("AgentMetrics() uptime_sec is not int64")
	}
	// Uptime should be approximately 1 hour (3600 seconds)
	if uptimeSec < 3500 || uptimeSec > 3700 {
		t.Errorf("AgentMetrics() uptime_sec = %v, want ~3600", uptimeSec)
	}
}

func TestAgentMetrics_TimestampFormat(t *testing.T) {
	svcCtx := newTestServiceContext()
	now := time.Now()
	svcCtx.SetRegisteredAt(now)
	svcCtx.SetLastHeartbeat(now)

	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	registeredAt, ok := resp.Metrics["registered_at"].(string)
	if !ok {
		t.Fatal("AgentMetrics() registered_at is not string")
	}
	if registeredAt == "" {
		t.Error("AgentMetrics() registered_at is empty")
	}

	// Verify RFC3339 format
	_, err = time.Parse(time.RFC3339, registeredAt)
	if err != nil {
		t.Errorf("AgentMetrics() registered_at is not RFC3339 format: %v", err)
	}
}

func TestAgentMetrics_EmptyTimestamps(t *testing.T) {
	svcCtx := newTestServiceContext()
	// Don't set timestamps - they should be zero

	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	registeredAt, ok := resp.Metrics["registered_at"].(string)
	if !ok {
		t.Fatal("AgentMetrics() registered_at is not string")
	}
	if registeredAt != "" {
		t.Errorf("AgentMetrics() registered_at = %v, want empty for zero time", registeredAt)
	}
}

func TestAgentMetrics_ConfiguredAddresses(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	grpcAddr := resp.Metrics["grpc_addr"]
	if grpcAddr != "localhost:19090" {
		t.Errorf("AgentMetrics() grpc_addr = %v, want 'localhost:19090'", grpcAddr)
	}

	upstreamAddr := resp.Metrics["upstream_addr"]
	if upstreamAddr != "localhost:8443" {
		t.Errorf("AgentMetrics() upstream_addr = %v, want 'localhost:8443'", upstreamAddr)
	}
}

func TestAgentMetrics_HeartbeatInterval(t *testing.T) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() error = %v", err)
	}

	heartbeatSec := resp.Metrics["heartbeat_sec"]
	if heartbeatSec != int64(30) {
		t.Errorf("AgentMetrics() heartbeat_sec = %v, want 30", heartbeatSec)
	}
}

func TestAgentMetrics_Concurrent(t *testing.T) {
	svcCtx := newTestServiceContext()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logic := NewAgentMetricsLogic(nil, svcCtx)
			req := &types.AgentMetricsRequest{
				AgentId: "test-agent-001",
			}
			resp, err := logic.AgentMetrics(req)
			if err != nil {
				t.Errorf("Concurrent AgentMetrics() error = %v", err)
			}
			if resp == nil {
				t.Error("Concurrent AgentMetrics() returned nil response")
			}
		}()
	}

	wg.Wait()
}

// Table-driven tests for validation
func TestAgentMetrics_Validation(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		wantErr bool
	}{
		{"valid agent id", "test-agent-001", false},
		{"empty agent id", "", true},
		{"whitespace only", "   ", true},
		{"wrong agent id", "wrong-agent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcCtx := newTestServiceContext()
			logic := NewAgentMetricsLogic(nil, svcCtx)

			req := &types.AgentMetricsRequest{
				AgentId: tt.agentID,
			}

			_, err := logic.AgentMetrics(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AgentMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentMetrics_NoConfiguredAgentID(t *testing.T) {
	cfg := config.Config{}
	cfg.Agent.ID = "" // No configured ID
	cfg.Agent.GameID = "test-game"
	cfg.Agent.Env = "development"
	cfg.Server.Addr = "localhost:8443"
	cfg.Upstream.HeartbeatInterval = 30

	svcCtx := &svc.ServiceContext{
		Config:        cfg,
		StartTime:     time.Now(),
		Core:          nil,
		LocalGRPCAddr: "localhost:19090",
	}

	logic := NewAgentMetricsLogic(nil, svcCtx)

	req := &types.AgentMetricsRequest{
		AgentId: "any-agent-id",
	}

	// When no agent ID is configured, any ID should be accepted
	resp, err := logic.AgentMetrics(req)
	if err != nil {
		t.Errorf("AgentMetrics() with no configured ID error = %v", err)
	}
	if resp == nil {
		t.Error("AgentMetrics() returned nil response")
	}
}

func BenchmarkAgentMetrics(b *testing.B) {
	svcCtx := newTestServiceContext()
	logic := NewAgentMetricsLogic(nil, svcCtx)
	req := &types.AgentMetricsRequest{
		AgentId: "test-agent-001",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = logic.AgentMetrics(req)
	}
}
