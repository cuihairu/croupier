package openapi

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/function/converter"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

func TestPipeline_ProtoToOpenAPIToServerAPI(t *testing.T) {
	conv := converter.NewProtoConverter()
	op, err := conv.ProtoToOpenAPI(&converter.ProtoMethodInfo{
		Name:       "GetPlayer",
		Package:    "games.player.v1",
		Service:    "PlayerService",
		InputType:  "GetPlayerRequest",
		OutputType: "GetPlayerResponse",
	}, map[string]interface{}{
		"x-entity":    "Player",
		"x-operation": "read",
	})
	if err != nil {
		t.Fatalf("ProtoToOpenAPI failed: %v", err)
	}

	store := registry.NewStore()
	funcID := "PlayerService.GetPlayer"
	if err := store.UpsertOpenAPI(funcID, op); err != nil {
		t.Fatalf("UpsertOpenAPI failed: %v", err)
	}

	logic := NewFunctionOpenAPISpecLogic(context.Background(), &svc.ServiceContext{RegistryStore: store})
	resp, err := logic.FunctionOpenAPISpec(&types.OpenAPISpecRequest{ID: funcID})
	if err != nil {
		t.Fatalf("FunctionOpenAPISpec failed: %v", err)
	}

	spec, ok := resp.Spec.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected spec type: %T", resp.Spec)
	}
	if spec["operationId"] != funcID {
		t.Fatalf("unexpected operationId: %v", spec["operationId"])
	}
	if spec["x-entity"] != "Player" {
		t.Fatalf("unexpected x-entity: %v", spec["x-entity"])
	}
}

func TestPipeline_PackToOpenAPIToServerAPI(t *testing.T) {
	conv := converter.NewPackConverter()
	ops, err := conv.PackToOpenAPI(&converter.PackManifest{
		ID:      "player-pack",
		Version: "1.0.0",
		Name:    "Player Pack",
		Functions: []converter.PackFunction{
			{
				ID:        "player.ban",
				Name:      "Ban Player",
				Summary:   "Ban Player",
				Params:    map[string]interface{}{"type": "object", "properties": map[string]interface{}{"playerId": map[string]interface{}{"type": "string"}}},
				Returns:   map[string]interface{}{"type": "object", "properties": map[string]interface{}{"success": map[string]interface{}{"type": "boolean"}}},
				Entity:    "Player",
				Operation: "update",
			},
		},
	})
	if err != nil {
		t.Fatalf("PackToOpenAPI failed: %v", err)
	}

	store := registry.NewStore()
	for fid, op := range ops {
		if err := store.UpsertOpenAPI(fid, op); err != nil {
			t.Fatalf("UpsertOpenAPI(%s) failed: %v", fid, err)
		}
	}

	logic := NewEntityFunctionsLogic(context.Background(), &svc.ServiceContext{RegistryStore: store})
	resp, err := logic.EntityFunctions(&types.EntityFunctionsRequest{ID: "Player"})
	if err != nil {
		t.Fatalf("EntityFunctions failed: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 entity function, got %d", len(resp.Items))
	}
	if resp.Items[0].Id != "player.ban" {
		t.Fatalf("unexpected function id: %s", resp.Items[0].Id)
	}
	if resp.Items[0].Operation != "update" {
		t.Fatalf("unexpected operation: %s", resp.Items[0].Operation)
	}
}

func TestPipeline_OpenAPIProviderToServerAPI(t *testing.T) {
	logic := NewOpenAPIImportLogic(context.Background(), &svc.ServiceContext{RegistryStore: registry.NewStore()})
	importResp, err := logic.OpenAPIImport(&types.OpenAPIImportRequest{
		Spec: map[string]interface{}{
			"openapi": "3.0.3",
			"info": map[string]interface{}{
				"title":   "Test API",
				"version": "1.0.0",
			},
			"paths": map[string]interface{}{
				"/players": map[string]interface{}{
					"post": map[string]interface{}{
						"operationId": "createPlayer",
						"summary":     "Create Player",
						"x-entity":    "Player",
						"x-operation": "create",
						"responses": map[string]interface{}{
							"200": map[string]interface{}{
								"description": "ok",
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("OpenAPIImport returned error: %v", err)
	}
	if importResp.Imported != 1 {
		t.Fatalf("expected 1 imported operation, got %d, failed=%v", importResp.Imported, importResp.Failed)
	}

	specLogic := NewFunctionOpenAPISpecLogic(context.Background(), logic.svcCtx)
	specResp, err := specLogic.FunctionOpenAPISpec(&types.OpenAPISpecRequest{ID: "POST/players"})
	if err != nil {
		t.Fatalf("FunctionOpenAPISpec failed: %v", err)
	}

	spec, ok := specResp.Spec.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected spec type: %T", specResp.Spec)
	}
	if spec["operationId"] != "createPlayer" {
		t.Fatalf("unexpected operationId: %v", spec["operationId"])
	}
}
