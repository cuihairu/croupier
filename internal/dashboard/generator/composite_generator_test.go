package generator

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// 自由组合页：多函数区块 + page_state 联动 + selector 骨架。
func TestGenerateCompositePage_Sections(t *testing.T) {
	contracts := []*model.FunctionContract{
		{
			Model:        gorm.Model{ID: 101},
			FunctionID:   "player.get",
			ResourceKey:  "player",
			Capability:   dbenum.CapabilityItemQuery,
			Execution:    string(spec.FunctionExecutionSync),
			Enabled:      true,
			InputSchema:  model.JSON(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"player":{"type":"object"},"gold":{"type":"integer"}}}`),
		},
		{
			Model:        gorm.Model{ID: 301},
			FunctionID:   "order.list",
			ResourceKey:  "order",
			Capability:   dbenum.CapabilityCollectionQuery,
			Execution:    string(spec.FunctionExecutionSync),
			Enabled:      true,
			InputSchema:  model.JSON(`{"type":"object","properties":{"playerId":{"type":"string"}},"required":["playerId"]}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`),
		},
	}
	gen, ok := GenerateCompositePage("composite--player-overview", []CompositeSectionInput{
		{FunctionID: "player.get", View: "fields", AutoRun: false},
		{FunctionID: "order.list", View: "table", RefreshOn: []string{"player.get"}},
	}, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("should generate")
	}
	if gen.Type != spec.PageTypeComposite || len(gen.Composite.Sections) != 2 {
		t.Fatalf("sections = %+v", gen.Composite)
	}
	// binding 与 section key 对应
	byID := map[string]spec.PageFunctionBinding{}
	for _, b := range gen.Bindings {
		byID[b.ID] = b
	}
	if _, ok := byID["player.get"]; !ok {
		t.Fatalf("expected binding player.get, got %v", byID)
	}
	// 必填输入映射 page_state（联动）
	get := byID["player.get"]
	found := false
	for _, a := range get.Selectors.Input.Assignments {
		if a.Target == "/id" && a.Source.Kind == spec.SourcePageState && a.Source.Key == "player.get" {
			found = true
		}
	}
	if !found {
		t.Fatalf("player.get required input should map page_state, got %+v", get.Selectors)
	}
	// 输出写 stateKey（供下游消费）
	list := byID["order.list"]
	if len(list.Selectors.Output) == 0 || list.Selectors.Output[0].StateKey != "order.list" {
		t.Fatalf("output should write stateKey=order.list, got %+v", list.Selectors.Output)
	}
	// 缺函数诊断
	if !strings.Contains(gen.Composite.Sections[1].Key, "order") {
		t.Fatalf("section key unexpected: %s", gen.Composite.Sections[1].Key)
	}
}

// 全部函数缺失 → false。
func TestGenerateCompositePage_AllMissing(t *testing.T) {
	if _, ok := GenerateCompositePage("x", []CompositeSectionInput{{FunctionID: "nope"}}, nil, DefaultGenerateOptions()); ok {
		t.Fatal("all missing should not generate")
	}
}
