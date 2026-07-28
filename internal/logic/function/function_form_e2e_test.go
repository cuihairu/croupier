package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func testFormilySchema(component string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"reason": map[string]interface{}{
				"type":        "string",
				"title":       "Reason",
				"x-component": component,
				"x-decorator": "FormItem",
			},
		},
		"required": []interface{}{"reason"},
	}
}

func TestFunctionForm_EndToEndOverride(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Function{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "player.ban",
		SpecFormat: "openapi3.0.3",
		OpenAPISpec: datatypes.JSONMap{
			"x-ui": testFormilySchema("Input.TextArea"),
		},
	}
	if err := db.Create(fn).Error; err != nil {
		t.Fatalf("create function failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Config:        config.Config{},
		FunctionModel: model.NewFunctionModel(db),
	}

	// Step 1: OpenAPI x-ui is not a valid function registration source.
	getLogic := NewFunctionFormLogic(context.Background(), svcCtx)
	getResp, err := getLogic.FunctionForm(&FunctionFormRequest{ID: "player.ban"})
	if err != nil {
		t.Fatalf("FunctionForm failed: %v", err)
	}
	if getResp.FormSource != "generated_default" {
		t.Fatalf("expected formSource=generated_default, got %s", getResp.FormSource)
	}

	// Step 2: apply custom override.
	updateLogic := NewFunctionFormUpdateLogic(context.Background(), svcCtx)
	updateResp, err := updateLogic.FunctionFormUpdate(&FunctionFormUpdateRequest{
		ID:     "player.ban",
		Schema: rawJSONFromValue(testFormilySchema("Select")),
	})
	if err != nil {
		t.Fatalf("FunctionFormUpdate failed: %v", err)
	}
	if updateResp.FormSource != "custom_metadata" {
		t.Fatalf("expected formSource=custom_metadata after update, got %s", updateResp.FormSource)
	}
}

func TestFunctionForm_RuntimeOnlyFunctionFallback(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Function{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Config:        config.Config{},
		FunctionModel: model.NewFunctionModel(db),
	}

	logic := NewFunctionFormLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionForm(&FunctionFormRequest{ID: "examples.analytics.player_retention"})
	if err != nil {
		t.Fatalf("FunctionForm fallback failed: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}
	if resp.FormSource != "generated_default" {
		t.Fatalf("expected formSource=generated_default for runtime-only function, got %s", resp.FormSource)
	}
	if resp.Schema == nil {
		t.Fatalf("expected generated schema, got nil")
	}
}
