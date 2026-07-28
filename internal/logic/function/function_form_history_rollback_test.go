package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func historyTestFormilySchema(component string) map[string]interface{} {
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
	}
}

func TestFunctionForm_HistoryAndRollback(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Function{}, &model.ConfigVersion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	fn := &model.Function{
		FunctionID: "player.ban",
		Name:       "player.ban",
	}
	if err := db.Create(fn).Error; err != nil {
		t.Fatalf("create function failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Config:             config.Config{},
		FunctionModel:      model.NewFunctionModel(db),
		ConfigVersionModel: model.NewConfigVersionModel(db),
	}

	updateLogic := NewFunctionFormUpdateLogic(context.Background(), svcCtx)
	_, err = updateLogic.FunctionFormUpdate(&FunctionFormUpdateRequest{
		ID:     "player.ban",
		Schema: rawJSONFromValue(historyTestFormilySchema("Input.TextArea")),
	})
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	_, err = updateLogic.FunctionFormUpdate(&FunctionFormUpdateRequest{
		ID:     "player.ban",
		Schema: rawJSONFromValue(historyTestFormilySchema("Select")),
	})
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	historyLogic := NewFunctionFormHistoryLogic(context.Background(), svcCtx)
	historyResp, err := historyLogic.FunctionFormHistory(&FunctionFormHistoryRequest{ID: "player.ban"})
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	if len(historyResp.Items) < 2 {
		t.Fatalf("expected at least 2 history entries, got %d", len(historyResp.Items))
	}

	rollbackLogic := NewFunctionFormRollbackLogic(context.Background(), svcCtx)
	rollbackResp, err := rollbackLogic.FunctionFormRollback(&FunctionFormRollbackRequest{
		ID:      "player.ban",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	currentUI := rollbackResp.Current
	if currentUI == nil {
		t.Fatalf("current should not be nil")
	}
	currentSchema, err := jsonObjectFromRaw(currentUI.Schema)
	if err != nil {
		t.Fatalf("current schema should be valid object JSON: %v", err)
	}
	props, _ := currentSchema["properties"].(map[string]interface{})
	reason, _ := props["reason"].(map[string]interface{})
	if reason["x-component"] != "Input.TextArea" {
		t.Fatalf("expected rolled back x-component=Input.TextArea, got %#v", reason["x-component"])
	}
}
