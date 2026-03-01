package nng

import (
	"context"
	"testing"

	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	componentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/component/v1"
)

func TestHandleRegisterRequest_SyncsOpenAPIOperation(t *testing.T) {
	server := NewServer(":0", nil)
	req := &agentv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:          "player.ban",
				Version:     "1.0.0",
				Category:    "player",
				Entity:      "Player",
				Operation:   "update",
				DisplayName: &componentv1.I18NText{En: "Ban Player"},
				Summary:     &componentv1.I18NText{En: "Permanently ban player"},
				InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}}}`,
			},
		},
	}

	if _, err := server.handleRegisterRequest(context.Background(), req); err != nil {
		t.Fatalf("handleRegisterRequest failed: %v", err)
	}

	op, err := server.Store().GetOpenAPI("player.ban")
	if err != nil {
		t.Fatalf("expected synced openapi operation, got error: %v", err)
	}
	if op.OperationID != "player.ban" {
		t.Fatalf("unexpected operation id: %s", op.OperationID)
	}
	if op.Summary != "Ban Player" {
		t.Fatalf("unexpected summary: %s", op.Summary)
	}
	if ext, ok := op.Extensions["x-entity"].(string); !ok || ext != "Player" {
		t.Fatalf("unexpected x-entity extension: %#v", op.Extensions["x-entity"])
	}
}

func TestHandleRegisterRequest_InvalidSchemaDoesNotFailRegistration(t *testing.T) {
	server := NewServer(":0", nil)
	req := &agentv1.RegisterRequest{
		AgentId: "agent-1",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{
				Id:          "player.bad",
				Version:     "1.0.0",
				DisplayName: &componentv1.I18NText{En: "Bad"},
				InputSchema: `{"type":"object","properties":{"x":{"type":"string"}`,
			},
		},
	}

	if _, err := server.handleRegisterRequest(context.Background(), req); err != nil {
		t.Fatalf("handleRegisterRequest should not fail on invalid schema: %v", err)
	}
	if _, err := server.Store().GetOpenAPI("player.bad"); err == nil {
		t.Fatal("expected no openapi operation for invalid schema")
	}
}
