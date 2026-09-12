package service

import (
	"context"
	"encoding/json"
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

// Group 弹窗分组必须透传到生成 spec：渲染端 openDialog 按 group 聚合
// dialog 区块（PageRenderer groupOf），service 转换层漏传会让弹窗永远打不开。
func TestCreateCompositeProposal_GroupPassthrough(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/group.db"), &gorm.Config{})
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

	proposal, err := svc.CreateCompositeProposal(ctx, "demo_game", "development", "composite--group-pass", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Display: "dialog", Group: "mailModal"},
		{FunctionID: "player.get", View: "fields", Key: "playerDetail", Title: "玩家"},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var page spec.PageSpec
	if err := jsonUnmarshalV9(proposal.PageSpec, &page); err != nil {
		t.Fatalf("unmarshal pageSpec: %v", err)
	}
	if len(page.Composite.Sections) != 2 {
		t.Fatalf("sections = %+v", page.Composite.Sections)
	}
	if got := page.Composite.Sections[0].Group; got != "mailModal" {
		t.Fatalf("group lost in generated spec: got %q, want %q", got, "mailModal")
	}
}

// TestCreateCompositeProposal_TabPassthrough V2 页签容器：display=tab 与
// group/tab 必须透传到生成 spec（fn 区块经生成器、static 区块在 service
// 手动落位）——渲染端按 group→Tabs、tab→页聚合，漏传会让页签页散落平铺。
func TestCreateCompositeProposal_TabPassthrough(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/tab.db"), &gorm.Config{})
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

	proposal, err := svc.CreateCompositeProposal(ctx, "demo_game", "development", "composite--tab-pass", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Display: "tab", Group: "mainTabs", Tab: "详情页"},
		{
			Key: "filter-panel", Static: true, View: "form", Title: "筛选",
			Display: "tab", Group: "mainTabs", Tab: "筛选页",
			Form: &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"kw":{"type":"string"}}}`)},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var page spec.PageSpec
	if err := jsonUnmarshalV9(proposal.PageSpec, &page); err != nil {
		t.Fatalf("unmarshal pageSpec: %v", err)
	}
	if len(page.Composite.Sections) != 2 {
		t.Fatalf("sections = %+v", page.Composite.Sections)
	}
	byKey := map[string]spec.CompositeSection{}
	for _, s := range page.Composite.Sections {
		byKey[s.Key] = s
	}
	fnSec := byKey["player.get"]
	if fnSec.Display != "tab" {
		t.Fatalf("fn section display = %q, want tab", fnSec.Display)
	}
	if fnSec.Group != "mainTabs" {
		t.Fatalf("fn section group = %q, want mainTabs", fnSec.Group)
	}
	if got := fnSec.Tab["zh-CN"]; got != "详情页" {
		t.Fatalf("fn section tab = %q, want 详情页", got)
	}
	staticSec := byKey["filter-panel"]
	if staticSec.Display != "tab" {
		t.Fatalf("static section display = %q, want tab", staticSec.Display)
	}
	if staticSec.Group != "mainTabs" {
		t.Fatalf("static section group = %q, want mainTabs", staticSec.Group)
	}
	if got := staticSec.Tab["zh-CN"]; got != "筛选页" {
		t.Fatalf("static section tab = %q, want 筛选页", got)
	}
}

// TestCreateCompositeProposal_VisibleWhenPassthrough U10 区块级条件显示：
// visibleWhen 必须透传到生成 spec（fn 区块经生成器、static 区块在 service
// 手动落位）——渲染端 sectionVisible 按叶子 key+path 求值，漏传会让条件
// 区块恒显/恒隐。
func TestCreateCompositeProposal_VisibleWhenPassthrough(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/visible-when.db"), &gorm.Config{})
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
	if err := svc.RebuildContractFromFunctionMeta(ctx, "demo_game", "development", "agent-1", spec.FunctionContractInput{ID: "player.get", Resource: "player", Capability: "item_query", Execution: "sync", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`, OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`}); err != nil {
		t.Fatal(err)
	}

	cond := &spec.ConditionSpec{
		Kind:  "equals",
		Key:   "filter-panel",
		Path:  "/values/mode",
		Value: json.RawMessage(`"advanced"`),
	}
	proposal, err := svc.CreateCompositeProposal(ctx, "demo_game", "development", "composite--visible-when", []CompositeSectionRequest{
		{
			Key: "player.get", FunctionID: "player.get", View: "fields",
			VisibleWhen: cond,
		},
		{
			Key: "filter-panel", Static: true, View: "form", Title: "筛选",
			Form:        &spec.FormPresentationSpec{JSONSchema: spec.JSONSchema(`{"type":"object","properties":{"mode":{"type":"string"}}}`)},
			VisibleWhen: &spec.ConditionSpec{Kind: "exists", Key: "player.get", Path: "/data/player"},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	var page spec.PageSpec
	if err := jsonUnmarshalV9(proposal.PageSpec, &page); err != nil {
		t.Fatalf("unmarshal pageSpec: %v", err)
	}
	if len(page.Composite.Sections) != 2 {
		t.Fatalf("sections = %+v", page.Composite.Sections)
	}
	byKey := map[string]spec.CompositeSection{}
	for _, s := range page.Composite.Sections {
		byKey[s.Key] = s
	}
	fnCond := byKey["player.get"].VisibleWhen
	if fnCond == nil {
		t.Fatal("fn section visibleWhen lost")
	}
	if fnCond.Kind != "equals" || fnCond.Key != "filter-panel" || fnCond.Path != "/values/mode" {
		t.Fatalf("fn section condition wrong: %+v", fnCond)
	}
	if string(fnCond.Value) != `"advanced"` {
		t.Fatalf("fn section condition value = %s", fnCond.Value)
	}
	staticCond := byKey["filter-panel"].VisibleWhen
	if staticCond == nil {
		t.Fatal("static section visibleWhen lost")
	}
	if staticCond.Kind != "exists" || staticCond.Key != "player.get" || staticCond.Path != "/data/player" {
		t.Fatalf("static section condition wrong: %+v", staticCond)
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
