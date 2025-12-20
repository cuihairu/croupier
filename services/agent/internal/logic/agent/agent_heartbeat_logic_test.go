package agent

import (
	"context"
	"strings"
	"testing"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
)

func TestAgentHeartbeatLogic_AgentHeartbeat(t *testing.T) {
	t.Parallel()

	makeSvcCtx := func(configuredID string, heartbeatInterval int64, coreRunning bool) *svc.ServiceContext {
		cfg := config.Config{}
		cfg.Agent.ID = configuredID
		cfg.Upstream.HeartbeatInterval = heartbeatInterval

		var core *agentcore.App
		if coreRunning {
			core = agentcore.New("", configuredID)
		}

		return svc.NewServiceContext(cfg, core, "127.0.0.1:19090")
	}

	t.Run("rejects nil request", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentHeartbeatLogic(context.Background(), makeSvcCtx("agent-1", 30, true))
		_, err := logic.AgentHeartbeat(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects missing agent id", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentHeartbeatLogic(context.Background(), makeSvcCtx("agent-1", 30, true))
		_, err := logic.AgentHeartbeat(&types.AgentHeartbeatRequest{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects mismatch with configured id", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentHeartbeatLogic(context.Background(), makeSvcCtx("agent-1", 30, true))
		_, err := logic.AgentHeartbeat(&types.AgentHeartbeatRequest{AgentId: "agent-2"})
		if err == nil || !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("returns configured interval when set", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentHeartbeatLogic(context.Background(), makeSvcCtx("agent-1", 15, false))
		resp, err := logic.AgentHeartbeat(&types.AgentHeartbeatRequest{AgentId: "agent-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextHeartbeat != 15 {
			t.Fatalf("expected next heartbeat 15, got %d", resp.NextHeartbeat)
		}
	})

	t.Run("defaults heartbeat interval to 30", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentHeartbeatLogic(context.Background(), makeSvcCtx("agent-1", 0, false))
		resp, err := logic.AgentHeartbeat(&types.AgentHeartbeatRequest{AgentId: "agent-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextHeartbeat != 30 {
			t.Fatalf("expected next heartbeat 30, got %d", resp.NextHeartbeat)
		}
	})
}
