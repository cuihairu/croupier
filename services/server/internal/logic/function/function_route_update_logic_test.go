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

func TestFunctionRouteUpdate_RuntimeOnlyFunction(t *testing.T) {
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

	logic := NewFunctionRouteUpdateLogic(context.Background(), svcCtx)
	resp, err := logic.FunctionRouteUpdate(&types.FunctionRouteUpdateRequest{
		ID:     "examples.analytics.player_retention",
		Nodes:  []string{"game", "player"},
		Path:   "/game/entities/player",
		Order:  100,
		Hidden: false,
	})
	if err != nil {
		t.Fatalf("FunctionRouteUpdate failed: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}
	if resp.Source != "metadata" {
		t.Fatalf("expected source=metadata, got %s", resp.Source)
	}

	saved, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), "examples.analytics.player_retention")
	if err != nil {
		t.Fatalf("expected function record persisted: %v", err)
	}
	if saved.Metadata == nil {
		t.Fatalf("expected metadata persisted, got nil")
	}
}
