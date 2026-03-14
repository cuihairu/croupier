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

func TestFunctionUI_EndToEndOverride(t *testing.T) {
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
			"x-ui": map[string]interface{}{
				"fields": map[string]interface{}{
					"reason": map[string]interface{}{"widget": "textarea"},
				},
			},
		},
	}
	if err := db.Create(fn).Error; err != nil {
		t.Fatalf("create function failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Config:        config.Config{},
		FunctionModel: model.NewFunctionModel(db),
	}

	// Step 1: default source should come from OpenAPI x-ui.
	getLogic := NewFunctionUILogicV2(context.Background(), svcCtx)
	getResp, err := getLogic.FunctionUI(&FunctionUIRequest{ID: "player.ban"})
	if err != nil {
		t.Fatalf("FunctionUI failed: %v", err)
	}
	if getResp.UISource != "openapi_x_ui" {
		t.Fatalf("expected uiSource=openapi_x_ui, got %s", getResp.UISource)
	}

	// Step 2: apply custom override.
	updateLogic := NewFunctionUIUpdateLogic(context.Background(), svcCtx)
	updateResp, err := updateLogic.FunctionUIUpdate(&FunctionUIUpdateRequest{
		ID: "player.ban",
		Schema: map[string]interface{}{
			"fields": map[string]interface{}{
				"reason": map[string]interface{}{"widget": "select"},
			},
		},
	})
	if err != nil {
		t.Fatalf("FunctionUIUpdate failed: %v", err)
	}
	if updateResp.UISource != "custom_metadata" {
		t.Fatalf("expected uiSource=custom_metadata after update, got %s", updateResp.UISource)
	}
}

func TestFunctionUI_RuntimeOnlyFunctionFallback(t *testing.T) {
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

	logic := NewFunctionUILogicV2(context.Background(), svcCtx)
	resp, err := logic.FunctionUI(&FunctionUIRequest{ID: "examples.analytics.player_retention"})
	if err != nil {
		t.Fatalf("FunctionUI fallback failed: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}
	if resp.UISource != "none" {
		t.Fatalf("expected uiSource=none for runtime-only function, got %s", resp.UISource)
	}
}
