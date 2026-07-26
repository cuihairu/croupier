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

func TestDescriptorsV2_OpenAPIOperationProvidesParams(t *testing.T) {
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
			"x-resource":   "player",
			"x-risk":       "high",
			"x-operation":  "ban",
			"x-permission": "player.ban",
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
	result, err := logic.DescriptorsV2(&DescriptorsRequest{})
	if err != nil {
		t.Fatalf("descriptors v2 failed: %v", err)
	}
	if len(result.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Functions))
	}
	fn := result.Functions[0]
	if fn.ID != "player.ban" {
		t.Fatalf("unexpected id: %v", fn.ID)
	}
	if fn.Risk != "high" {
		t.Fatalf("unexpected risk: %v", fn.Risk)
	}
	if fn.Resource != "player" {
		t.Fatalf("unexpected resource: %v", fn.Resource)
	}
	if fn.Operation != "ban" {
		t.Fatalf("unexpected operation: %v", fn.Operation)
	}
	if fn.Permission != "player.ban" {
		t.Fatalf("unexpected permission: %v", fn.Permission)
	}
	if fn.Description == nil || !strings.Contains(fn.Description["en-US"], "Ban a player account") {
		t.Fatalf("unexpected description: %v", fn.Description)
	}
	if fn.Summary == nil || !strings.Contains(fn.Summary["en-US"], "Ban") {
		t.Fatalf("unexpected summary: %v", fn.Summary)
	}
	if fn.InputSchema == nil {
		t.Fatalf("inputSchema should not be nil")
	}
	if !strings.Contains(string(fn.InputSchema), "player_id") {
		t.Fatalf("expected inputSchema to include player_id, got %q", string(fn.InputSchema))
	}
	if fn.OutputSchema == nil {
		t.Fatalf("outputSchema should not be nil")
	}
	if !strings.Contains(string(fn.OutputSchema), "success") {
		t.Fatalf("expected outputSchema to include success, got %q", string(fn.OutputSchema))
	}
}

func TestDescriptorsV2_DoesNotInferPageSemanticsFromFunctionID(t *testing.T) {
	store := reg.NewStore()
	if err := store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		Summary:     "Ban Player",
		Description: "Ban a player account",
		Extensions: map[string]interface{}{
			"x-risk":      "high",
			"x-operation": "ban",
		},
	}); err != nil {
		t.Fatalf("upsert openapi failed: %v", err)
	}

	logic := NewDescriptorsLogic(context.Background(), &svc.ServiceContext{RegistryStore: store})
	result, err := logic.DescriptorsV2(&DescriptorsRequest{})
	if err != nil {
		t.Fatalf("descriptors v2 failed: %v", err)
	}
	if len(result.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.Resource != "" {
		t.Fatalf("resource must not be inferred from function id, got %q", fn.Resource)
	}
	if len(fn.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for missing resource")
	}
	codes := map[string]bool{}
	for _, diag := range fn.Diagnostics {
		codes[diag.Code] = true
	}
	if !codes["resource_missing"] {
		t.Fatalf("expected missing resource diagnostics, got %#v", fn.Diagnostics)
	}
}
