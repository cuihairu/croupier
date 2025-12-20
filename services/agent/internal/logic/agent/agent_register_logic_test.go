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

func TestAgentRegisterLogic_AgentRegister(t *testing.T) {
	t.Parallel()

	makeSvcCtx := func(configuredID string, coreRunning bool) *svc.ServiceContext {
		cfg := config.Config{}
		cfg.Agent.ID = configuredID
		cfg.Agent.LocalAddr = "127.0.0.1:19090"

		var core *agentcore.App
		if coreRunning {
			core = agentcore.New("", configuredID)
		}

		return svc.NewServiceContext(cfg, core, "127.0.0.1:19090")
	}

	t.Run("rejects nil request", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentRegisterLogic(context.Background(), makeSvcCtx("agent-1", true))
		_, err := logic.AgentRegister(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects when core missing", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentRegisterLogic(context.Background(), makeSvcCtx("agent-1", false))
		_, err := logic.AgentRegister(&types.AgentRegisterRequest{AgentId: "agent-1"})
		if err == nil || !strings.Contains(err.Error(), "core") {
			t.Fatalf("expected core error, got %v", err)
		}
	})

	t.Run("rejects missing agent id", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentRegisterLogic(context.Background(), makeSvcCtx("", true))
		_, err := logic.AgentRegister(&types.AgentRegisterRequest{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects mismatch with configured id", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentRegisterLogic(context.Background(), makeSvcCtx("agent-1", true))
		_, err := logic.AgentRegister(&types.AgentRegisterRequest{AgentId: "agent-2"})
		if err == nil || !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("expected mismatch error, got %v", err)
		}
	})

	t.Run("accepts id from config when request empty", func(t *testing.T) {
		t.Parallel()
		logic := NewAgentRegisterLogic(context.Background(), makeSvcCtx("agent-1", true))
		resp, err := logic.AgentRegister(&types.AgentRegisterRequest{
			GameId: "game-1",
			Env:    "staging",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil || !resp.Success {
			t.Fatalf("expected success response, got %+v", resp)
		}
		if !strings.HasPrefix(resp.Token, "agent-1:") {
			t.Fatalf("expected token prefix, got %q", resp.Token)
		}
	})
}
