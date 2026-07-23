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
			"x-category":          "ops",
			"x-risk":              "high",
			"x-entity":            "player",
			"x-operation":         "ban",
			"x-operation-kind":    "action",
			"x-placement":         "rowAction",
			"x-category-display":  map[string]interface{}{"zh-CN": "运营", "en-US": "Operations"},
			"x-entity-display":    map[string]interface{}{"zh-CN": "玩家", "en-US": "Player"},
			"x-operation-display": map[string]interface{}{"zh-CN": "封禁", "en-US": "Ban"},
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
	if fn.Category != "ops" {
		t.Fatalf("unexpected category: %v", fn.Category)
	}
	if fn.Risk != "high" {
		t.Fatalf("unexpected risk: %v", fn.Risk)
	}
	if fn.Entity != "player" {
		t.Fatalf("unexpected entity: %v", fn.Entity)
	}
	if fn.Operation != "ban" {
		t.Fatalf("unexpected operation: %v", fn.Operation)
	}
	if fn.OperationKind != "action" {
		t.Fatalf("unexpected operationKind: %v", fn.OperationKind)
	}
	if fn.Placement != "rowAction" {
		t.Fatalf("unexpected placement: %v", fn.Placement)
	}
	if fn.CategoryDisplay["zh-CN"] != "运营" || fn.EntityDisplay["zh-CN"] != "玩家" || fn.OperationDisplay["zh-CN"] != "封禁" {
		t.Fatalf("unexpected localized labels: category=%v entity=%v operation=%v", fn.CategoryDisplay, fn.EntityDisplay, fn.OperationDisplay)
	}
	if fn.Description == nil || !strings.Contains(fn.Description["en-US"], "Ban a player account") {
		t.Fatalf("unexpected description: %v", fn.Description)
	}
	if fn.Summary == nil || !strings.Contains(fn.Summary["en-US"], "Ban") {
		t.Fatalf("unexpected summary: %v", fn.Summary)
	}
	if fn.DisplayName == nil || !strings.Contains(fn.DisplayName["en-US"], "Ban") {
		t.Fatalf("unexpected displayName: %v", fn.DisplayName)
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
			"x-category":  "ops",
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
	if fn.Entity != "" {
		t.Fatalf("entity must not be inferred from function id, got %q", fn.Entity)
	}
	if fn.OperationKind != "" || fn.Placement != "" {
		t.Fatalf("page semantics must not be inferred, got kind=%q placement=%q", fn.OperationKind, fn.Placement)
	}
	if len(fn.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for missing v2 page semantics")
	}
	codes := map[string]bool{}
	for _, diag := range fn.Diagnostics {
		codes[diag.Code] = true
	}
	if !codes["operation_kind_missing"] || !codes["placement_missing"] {
		t.Fatalf("expected missing kind/placement diagnostics, got %#v", fn.Diagnostics)
	}
}
