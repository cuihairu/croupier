package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFunctionHistoryAndAnalytics_RuntimeOnly(t *testing.T) {
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

	historyLogic := NewFunctionHistoryLogic(context.Background(), svcCtx)
	items, err := historyLogic.FunctionHistory(&types.FunctionHistoryRequest{ID: "examples.analytics.player_retention"})
	if err != nil {
		t.Fatalf("FunctionHistory failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one history entry, got %d", len(items))
	}

	analyticsLogic := NewFunctionAnalyticsLogic(context.Background(), svcCtx)
	stats, err := analyticsLogic.FunctionAnalytics(&types.FunctionAnalyticsRequest{ID: "examples.analytics.player_retention"})
	if err != nil {
		t.Fatalf("FunctionAnalytics failed: %v", err)
	}
	if stats == nil || stats.TotalCalls != 0 {
		t.Fatalf("unexpected analytics response: %+v", stats)
	}
}
