package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// 复现组合页创建 panic：全模型 sqlite 环境（元数据库形状与生产一致）。
func TestCreateCompositeProposal_Repro(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.FunctionContract{},
		&model.CapabilitySemantics{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.TermDictionary{},
	); err != nil {
		t.Fatal(err)
	}
	svc := NewContractService(db)

	ctx := context.Background()
	// 种契约（player.get + order.list，形状与线上一致）
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo_game", "development", "agent-1", spec.FunctionContractInput{ID: "player.get", Resource: "player", Capability: "item_query", Execution: "sync", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: `{"type":"object","properties":{"player":{"type":"object"},"gold":{"type":"integer"}}}`}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo_game", "development", "agent-1", spec.FunctionContractInput{ID: "order.list", Resource: "order", Capability: "collection_query", Execution: "sync", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`, OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`}); err != nil {
		t.Fatal(err)
	}

	proposal, err := svc.CreateCompositeProposal(ctx, "demo_game", "development", "composite--player-overview", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{FunctionID: "order.list", View: "table", Title: "订单", RefreshOn: []string{"player.get"}},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if proposal == nil || proposal.ProposalKey == "" {
		t.Fatalf("proposal = %+v", proposal)
	}
}
