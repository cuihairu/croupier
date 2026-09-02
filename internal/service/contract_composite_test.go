package service

import (
	"context"
	"math"
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

// 生产路由形态：ctx 注入 per-game DB 覆盖（GameDBMiddleware 路径）。
func TestCreateCompositeProposal_ScopedCtxRepro(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/scoped.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.FunctionContract{},
		&model.CapabilitySemantics{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.TermDictionary{},
		&model.ResourceCapability{},
		&model.CapabilitySemanticVersion{},
		&model.BlockedProposalIssue{},
	); err != nil {
		t.Fatal(err)
	}
	svc := NewContractService(db)

	ctx := context.Background()
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo_game", "development", "agent-1", spec.FunctionContractInput{ID: "player.get", Resource: "player", Capability: "item_query", Execution: "sync", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo_game", "development", "agent-1", spec.FunctionContractInput{ID: "order.list", Resource: "order", Capability: "collection_query", Execution: "sync", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`, OutputSchema: `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`}); err != nil {
		t.Fatal(err)
	}

	proposal, err := svc.CreateCompositeProposal(ctx, "demo_game", "development", "composite--scoped", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields"},
		{FunctionID: "order.list", View: "table", RefreshOn: []string{"player.get"}},
	})
	if err != nil {
		t.Fatalf("scoped create failed: %v", err)
	}
	if proposal == nil || proposal.PageKey != "composite--scoped" {
		t.Fatalf("proposal = %+v", proposal)
	}
}

// TestConvChainAndEvents V3.2：请求动作链/事件绑定 → generator 输入透传。
func TestConvChainAndEvents(t *testing.T) {
	chain := convChain([]ActionStepReq{
		{Kind: "navigate", Params: map[string]string{"url": "/x"}},
		{Kind: "runBinding", Target: "player.list", Params: map[string]string{"playerId": "row.uid"}},
	})
	if len(chain) != 2 {
		t.Fatalf("chain lost: %+v", chain)
	}
	if chain[0].Kind != "navigate" || chain[0].Params["url"] != "/x" {
		t.Fatalf("chain[0] wrong: %+v", chain[0])
	}
	if chain[1].Target != "player.list" || chain[1].Params["playerId"] != "row.uid" {
		t.Fatalf("chain[1] wrong: %+v", chain[1])
	}

	evs := convEvents([]EventBindingReq{{
		Event:  "rowClick",
		Action: ActionStepReq{Kind: "openModal", Target: "modal-g1"},
		Chain:  []ActionStepReq{{Kind: "showMessage", Params: map[string]string{"message": "ok"}}},
	}})
	if len(evs) != 1 || evs[0].Event != "rowClick" {
		t.Fatalf("events lost: %+v", evs)
	}
	if evs[0].Action.Target != "modal-g1" {
		t.Fatalf("event action wrong: %+v", evs[0].Action)
	}
	if evs[0].Chain[0].Params["message"] != "ok" {
		t.Fatalf("event chain wrong: %+v", evs[0].Chain)
	}
}

// 声明式超时契约往返：输入 TimeoutMs → 契约列落库可读（执行层接线依赖）。
func TestContractTimeoutMsRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/meta.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.FunctionContract{}); err != nil {
		t.Fatal(err)
	}
	svc := NewContractService(db)
	ctx := context.Background()

	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo", "prod", "openapi", spec.FunctionContractInput{
		ID: "player.ban", Resource: "player", Capability: "update", Execution: "sync",
		Enabled: true, TimeoutMs: 25000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo", "prod", "openapi", spec.FunctionContractInput{
		ID: "player.query", Resource: "player", Capability: "item_query", Execution: "sync",
		Enabled: true, // 未声明 → 0
	}); err != nil {
		t.Fatal(err)
	}

	contractModel := model.NewFunctionContractModel(db)
	withTimeout, err := contractModel.FindByScopeAndFunctionID(ctx, "demo", "prod", "player.ban")
	if err != nil {
		t.Fatal(err)
	}
	if withTimeout.TimeoutMs != 25000 {
		t.Fatalf("timeout_ms = %d, want 25000", withTimeout.TimeoutMs)
	}
	without, err := contractModel.FindByScopeAndFunctionID(ctx, "demo", "prod", "player.query")
	if err != nil {
		t.Fatal(err)
	}
	if without.TimeoutMs != 0 {
		t.Fatalf("undeclared timeout_ms = %d, want 0", without.TimeoutMs)
	}

	// 再声明（重注册覆盖）：clamp 后的值更新落库
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo", "prod", "openapi", spec.FunctionContractInput{
		ID: "player.ban", Resource: "player", Capability: "update", Execution: "sync",
		Enabled: true, TimeoutMs: 300000, // 越界 → 60000
	}); err != nil {
		t.Fatal(err)
	}
	withTimeout, err = contractModel.FindByScopeAndFunctionID(ctx, "demo", "prod", "player.ban")
	if err != nil {
		t.Fatal(err)
	}
	if withTimeout.TimeoutMs != 60000 {
		t.Fatalf("clamped timeout_ms = %d, want 60000", withTimeout.TimeoutMs)
	}
}

// timeoutMsToInt32：收窄转换显式有界（CodeQL go/incorrect-integer-conversion）。
func TestTimeoutMsToInt32Bounds(t *testing.T) {
	if got := timeoutMsToInt32(-5); got != 0 {
		t.Fatalf("negative = %d, want 0", got)
	}
	if got := timeoutMsToInt32(25000); got != 25000 {
		t.Fatalf("normal = %d, want 25000", got)
	}
	if got := timeoutMsToInt32(1 << 40); got != math.MaxInt32 {
		t.Fatalf("overflow = %d, want MaxInt32", got)
	}
}
