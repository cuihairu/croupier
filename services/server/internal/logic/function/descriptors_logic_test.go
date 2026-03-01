package function

import (
	"context"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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
	err = store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		Summary: "Ban Player",
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
		Extensions: map[string]interface{}{
			"x-category": "player",
			"x-risk":     "high",
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
	items, err := logic.Descriptors(&types.DescriptorsRequest{})
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
	params, ok := item["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("params should be object, got %T", item["params"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok || props["player_id"] == nil {
		t.Fatalf("expected params.properties.player_id, got %#v", params["properties"])
	}
}
