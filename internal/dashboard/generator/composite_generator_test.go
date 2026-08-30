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

// TestCompositeRowActionsSurviveListView 行操作必须在 buildListView 重建
// section.Table 之后回填——此前顺序相反导致 rowActions 落库为 null
// （T4.5 生产实测：提案 spec table.rowActions 丢失）。
func TestCompositeRowActionsSurviveListView(t *testing.T) {
	contracts := []*model.FunctionContract{
		{
			FunctionID:   "player.list",
			ResourceKey:  "player",
			Capability:   dbenum.CapabilityCollectionQuery,
			Execution:    string(spec.FunctionExecutionSync),
			InputSchema:  model.JSON(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"uid":{"type":"string"},"gold":{"type":"number"}},"required":["uid"]}},"total":{"type":"integer"}}}`),
		},
	}
	inputs := []CompositeSectionInput{{
		FunctionID: "player.list",
		View:       "table",
		RowActions: []CompositeRowActionInput{{
			Label:         "发邮件",
			TargetSection: "mail.send",
			Params:        map[string]string{"playerId": "uid"},
		}},
	}}
	generated, ok := GenerateCompositePage("k", inputs, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("generate failed")
	}
	sec := generated.PageSpec.Composite.Sections[0]
	if len(sec.Table.Columns) == 0 {
		t.Fatal("columns lost after rowActions fix")
	}
	if len(sec.Table.RowActions) != 1 {
		t.Fatalf("rowActions lost: %+v", sec.Table)
	}
	if sec.Table.RowActions[0].Params["playerId"] != "uid" {
		t.Fatalf("params lost: %+v", sec.Table.RowActions[0])
	}
}

// TestCompositeEventsChainPassthrough V3.2：事件绑定与动作链（含 params）
// 从 Input 透传到发布 spec。
func TestCompositeEventsChainPassthrough(t *testing.T) {
	contracts := []*model.FunctionContract{
		{
			FunctionID:   "player.list",
			ResourceKey:  "player",
			Capability:   dbenum.CapabilityCollectionQuery,
			Execution:    string(spec.FunctionExecutionSync),
			InputSchema:  model.JSON(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}}}`),
		},
	}
	inputs := []CompositeSectionInput{{
		FunctionID: "player.list",
		View:       "table",
		Group:      "",
		Events: []spec.CompositeEventBinding{{
			Event:  "rowClick",
			Action: spec.CompositeActionStep{Kind: "openModal", Target: "modal-g1"},
			Chain: []spec.CompositeActionStep{{
				Kind:   "navigate",
				Params: map[string]string{"url": "/docs"},
			}},
		}},
		RowActions: []CompositeRowActionInput{{
			Label:         "行操作",
			TargetSection: "modal-g1",
			Chain: []spec.CompositeActionStep{{
				Kind:   "runBinding",
				Target: "player.list",
				Params: map[string]string{"playerId": "row.uid"},
			}},
		}},
	}}
	generated, ok := GenerateCompositePage("k", inputs, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("generate failed")
	}
	sec := generated.PageSpec.Composite.Sections[0]
	// Events 透传
	if len(sec.Events) != 1 || sec.Events[0].Event != "rowClick" {
		t.Fatalf("events passthrough lost: %+v", sec.Events)
	}
	if sec.Events[0].Action.Kind != "openModal" || sec.Events[0].Action.Target != "modal-g1" {
		t.Fatalf("event action lost: %+v", sec.Events[0].Action)
	}
	if sec.Events[0].Chain[0].Params["url"] != "/docs" {
		t.Fatalf("event chain params lost: %+v", sec.Events[0].Chain)
	}
	// 行操作 chain params 透传
	ra := sec.Table.RowActions[0]
	if len(ra.Chain) != 1 || ra.Chain[0].Params["playerId"] != "row.uid" {
		t.Fatalf("rowAction chain params lost: %+v", ra.Chain)
	}
}
