package function

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDescriptorsV2_OpenAPIOperationProvidesParams(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.FunctionContract{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	err = contractsvc.NewContractService(db).RebuildContractFromFunctionMeta(context.Background(), "demo-game", "development", "openapi", contractsvc.FunctionMetaInput{
		ID:           "player.ban",
		Version:      "1.0.0",
		Enabled:      true,
		Summary:      "Ban Player",
		Description:  "Ban a player account",
		InputSchema:  `{"type":"object","properties":{"player_id":{"type":"string"}},"required":["player_id"]}`,
		OutputSchema: `{"type":"object","properties":{"success":{"type":"boolean"}}}`,
		Resource:     "player",
		Operation:    "ban",
		Capability:   string(spec.CapabilityAction),
		Execution:    string(spec.FunctionExecutionSync),
		Risk:         string(spec.RiskHigh),
		Permission:   "player.ban",
	})
	if err != nil {
		t.Fatalf("rebuild contract failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		DB: db,
	}

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	logic := NewDescriptorsLogic(ctx, svcCtx)
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
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.FunctionContract{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	err = contractsvc.NewContractService(db).RebuildContractFromFunctionMeta(context.Background(), "demo-game", "development", "sdk", contractsvc.FunctionMetaInput{
		ID:          "player.ban",
		Version:     "1.0.0",
		Enabled:     true,
		Summary:     "Ban Player",
		Description: "Ban a player account",
		Operation:   "ban",
		Capability:  string(spec.CapabilityAction),
		Execution:   string(spec.FunctionExecutionSync),
		Risk:        string(spec.RiskHigh),
	})
	if err != nil {
		t.Fatalf("rebuild contract failed: %v", err)
	}
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo-game", Env: "development"})
	logic := NewDescriptorsLogic(ctx, &svc.ServiceContext{DB: db})
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
}
