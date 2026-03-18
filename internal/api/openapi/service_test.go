package openapi

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOpenAPITestService(t *testing.T) *Service {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	store := registry.NewStore()
	store.UpsertAgent(&registry.AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]registry.FunctionMeta{
			"player.list": {Enabled: true, Version: "1.0.0"},
		},
		LastSeen: time.Now(),
	})

	return NewService(&svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
		RegistryStore: store,
	})
}

func TestGetSpec_FallbackGeneratedOperation(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	resp, err := service.GetSpec(context.Background(), &GetSpecRequest{ID: "player.list"})
	if err != nil {
		t.Fatalf("GetSpec failed: %v", err)
	}
	if resp == nil || resp.Spec == nil {
		t.Fatal("expected generated spec")
	}

	op, ok := resp.Spec.(*openapi3.Operation)
	if !ok {
		t.Fatalf("expected *openapi3.Operation, got %T", resp.Spec)
	}
	if op.OperationID != "player.list" {
		t.Fatalf("unexpected operation id: %s", op.OperationID)
	}
}
