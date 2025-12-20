package agent

import (
	"context"
	"testing"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
	"github.com/cuihairu/croupier/services/agent/internal/config"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
)

func TestFunctionRegisterLogic_FunctionRegister(t *testing.T) {
	t.Parallel()

	makeSvcCtx := func(coreRunning bool) *svc.ServiceContext {
		cfg := config.Config{}
		cfg.Agent.ID = "agent-1"
		var core *agentcore.App
		if coreRunning {
			core = agentcore.New("", cfg.Agent.ID)
		}
		return svc.NewServiceContext(cfg, core, "127.0.0.1:19090")
	}

	t.Run("rejects nil request", func(t *testing.T) {
		t.Parallel()
		logic := NewFunctionRegisterLogic(context.Background(), makeSvcCtx(true))
		_, err := logic.FunctionRegister(nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("registers in core store when rpc_addr provided", func(t *testing.T) {
		t.Parallel()
		svcCtx := makeSvcCtx(true)
		logic := NewFunctionRegisterLogic(context.Background(), svcCtx)

		resp, err := logic.FunctionRegister(&types.FunctionRegisterRequest{
			FunctionId: "game.test/v1",
			Metadata: map[string]any{
				"rpc_addr":          "127.0.0.1:19090",
				"service_id":        "service-1",
				"service_version":   "sv1",
				"function_version":  "fv1",
				"unrelated_payload": "ignored",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil || !resp.Success {
			t.Fatalf("expected success response, got %+v", resp)
		}

		store := svcCtx.Core.Store()
		list := store.List()
		instances := list["game.test/v1"]
		if len(instances) != 1 {
			t.Fatalf("expected 1 instance, got %d", len(instances))
		}
		if instances[0].ServiceID != "service-1" || instances[0].Addr != "127.0.0.1:19090" || instances[0].Version != "sv1" {
			t.Fatalf("unexpected instance: %+v", instances[0])
		}

		versions := store.FunctionVersions()
		if got := versions["game.test/v1"]["service-1"]; got != "fv1" {
			t.Fatalf("expected function version fv1, got %q", got)
		}
	})

	t.Run("no-op when rpc_addr missing", func(t *testing.T) {
		t.Parallel()
		svcCtx := makeSvcCtx(true)
		logic := NewFunctionRegisterLogic(context.Background(), svcCtx)

		resp, err := logic.FunctionRegister(&types.FunctionRegisterRequest{
			FunctionId: "game.noop/v1",
			Metadata:   map[string]any{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil || !resp.Success {
			t.Fatalf("expected success response, got %+v", resp)
		}
		if len(svcCtx.Core.Store().List()) != 0 {
			t.Fatalf("expected store unchanged")
		}
	})
}
