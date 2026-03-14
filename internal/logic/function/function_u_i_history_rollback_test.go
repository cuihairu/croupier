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

func TestFunctionUI_HistoryAndRollback(t *testing.T) {
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

	updateLogic := NewFunctionUIUpdateLogic(context.Background(), svcCtx)
	_, err = updateLogic.FunctionUIUpdate(&FunctionUIUpdateRequest{
		ID:     "player.ban",
		Schema: map[string]interface{}{"fields": map[string]interface{}{"reason": map[string]interface{}{"widget": "textarea"}}},
	})
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	_, err = updateLogic.FunctionUIUpdate(&FunctionUIUpdateRequest{
		ID:     "player.ban",
		Schema: map[string]interface{}{"fields": map[string]interface{}{"reason": map[string]interface{}{"widget": "select"}}},
	})
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	historyLogic := NewFunctionUIHistoryLogic(context.Background(), svcCtx)
	historyResp, err := historyLogic.FunctionUIHistory(&FunctionUIHistoryRequest{ID: "player.ban"})
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	if len(historyResp.Items) < 2 {
		t.Fatalf("expected at least 2 history entries, got %d", len(historyResp.Items))
	}

	rollbackLogic := NewFunctionUIRollbackLogic(context.Background(), svcCtx)
	rollbackResp, err := rollbackLogic.FunctionUIRollback(&FunctionUIRollbackRequest{
		ID:      "player.ban",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	currentUI, ok := rollbackResp.Current.(*FunctionUIResponse)
	if !ok {
		t.Fatalf("current should be FunctionUIResponse, got %T", rollbackResp.Current)
	}
	currentSchema, ok := currentUI.Schema.(map[string]interface{})
	if !ok {
		t.Fatalf("current schema should be map, got %T", currentUI.Schema)
	}
	fields, _ := currentSchema["fields"].(map[string]interface{})
	reason, _ := fields["reason"].(map[string]interface{})
	if reason["widget"] != "textarea" {
		t.Fatalf("expected rolled back widget=textarea, got %#v", reason["widget"])
	}
}
