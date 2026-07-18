package function

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/getkin/kin-openapi/openapi3"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDescriptors_OpenAPIOperationProvidesParams(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Function{}, &model.Descriptor{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	store := reg.NewStore()
	objectType := openapi3.Types{"object"}
	stringType := openapi3.Types{"string"}
	boolType := openapi3.Types{"boolean"}
	responseDesc := "Ban response"
	err = store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		Summary:     "Ban Player",
		Description: "Ban a player account",
		Tags:        []string{"player", "moderation"},
		RequestBody: &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Content: map[string]*openapi3.MediaType{
					"application/json": {
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &objectType,
								Properties: map[string]*openapi3.SchemaRef{
									"player_id": {Value: &openapi3.Schema{Type: &stringType}},
								},
								Required: []string{"player_id"},
							},
						},
					},
				},
			},
		},
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", &openapi3.Response{
				Description: &responseDesc,
				Content: openapi3.Content{
					"application/json": {
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &objectType,
								Properties: map[string]*openapi3.SchemaRef{
									"success": {Value: &openapi3.Schema{Type: &boolType}},
								},
							},
						},
					},
				},
			}),
		),
		Extensions: map[string]interface{}{
			"x-category":  "player",
			"x-risk":      "high",
			"x-entity":    "player",
			"x-operation": "ban",
		},
	})
	if err != nil {
		t.Fatalf("upsert openapi failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		FunctionModel:   model.NewFunctionModel(db),
		RegistryStore:   store,
		PermissionModel: nil,
	}

	logic := NewDescriptorsLogic(context.Background(), svcCtx)
	items, err := logic.Descriptors(&DescriptorsRequest{})
	if err != nil {
		t.Fatalf("descriptors failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(items))
	}
	item := items[0]
	if item["id"] != "player.ban" {
		t.Fatalf("unexpected id: %v", item["id"])
	}
	if item["category"] != "player" {
		t.Fatalf("unexpected category: %v", item["category"])
	}
	if item["risk"] != "high" {
		t.Fatalf("unexpected risk: %v", item["risk"])
	}
	if item["entity"] != "player" {
		t.Fatalf("unexpected entity: %v", item["entity"])
	}
	if item["operation"] != "ban" {
		t.Fatalf("unexpected operation: %v", item["operation"])
	}
	if item["description"] != "Ban a player account" {
		t.Fatalf("unexpected description: %v", item["description"])
	}
	if summary, ok := item["summary"].(map[string]string); !ok || summary["en"] != "Ban a player account" {
		t.Fatalf("unexpected summary: %#v", item["summary"])
	}
	if displayName, ok := item["displayName"].(map[string]string); !ok || displayName["en"] != "Ban Player" {
		t.Fatalf("unexpected displayName: %#v", item["displayName"])
	}
	params, ok := item["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("params should be object, got %T", item["params"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok || props["player_id"] == nil {
		t.Fatalf("expected params.properties.player_id, got %#v", params["properties"])
	}
	if inputSchema, _ := item["inputSchema"].(string); !strings.Contains(inputSchema, "player_id") {
		t.Fatalf("expected inputSchema to include player_id, got %q", inputSchema)
	}
	outputs, ok := item["outputs"].(map[string]interface{})
	if !ok {
		t.Fatalf("outputs should be object, got %T", item["outputs"])
	}
	outputProps, ok := outputs["properties"].(map[string]interface{})
	if !ok || outputProps["success"] == nil {
		t.Fatalf("expected outputs.properties.success, got %#v", outputs["properties"])
	}
	if outputSchema, _ := item["outputSchema"].(string); !strings.Contains(outputSchema, "success") {
		t.Fatalf("expected outputSchema to include success, got %q", outputSchema)
	}
}
