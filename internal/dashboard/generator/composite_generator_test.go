package generator

import (
	"encoding/json"
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

// TestCompositeTabPassthrough V2 页签容器：display=tab 与 group/tab 从
// Input 透传到发布 spec（Tab 包装系统默认语言 LocalizedText，同 Title 模式）。
func TestCompositeTabPassthrough(t *testing.T) {
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
	inputs := []CompositeSectionInput{
		{FunctionID: "player.list", View: "table", Display: "tab", Group: "mainTabs", Tab: "列表页"},
		{FunctionID: "player.list", View: "table", Key: "vip.rank", Display: "tab", Group: "mainTabs", Tab: "VIP 页"},
	}
	generated, ok := GenerateCompositePage("k", inputs, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("generate failed")
	}
	for _, sec := range generated.PageSpec.Composite.Sections {
		if sec.Display != "tab" {
			t.Fatalf("display = %q, want tab (section %s)", sec.Display, sec.Key)
		}
		if sec.Group != "mainTabs" {
			t.Fatalf("group = %q, want mainTabs (section %s)", sec.Group, sec.Key)
		}
		want := "列表页"
		if sec.Key == "vip.rank" {
			want = "VIP 页"
		}
		if got := sec.Tab["zh-CN"]; got != want {
			t.Fatalf("tab = %q, want %q (section %s)", got, want, sec.Key)
		}
	}
}

// TestCompositeCardTitlePassthrough #94 卡片分组：display=card 与
// group/cardTitle 从 Input 透传到发布 spec（CardTitle 包装系统默认语言
// LocalizedText，同 Tab 模式）；未声明 cardTitle 的区块不携带该字段。
func TestCompositeCardTitlePassthrough(t *testing.T) {
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
	inputs := []CompositeSectionInput{
		{FunctionID: "player.list", View: "table", Display: "card", Group: "vip-zone", CardTitle: "VIP 专区"},
		{FunctionID: "player.list", View: "table", Key: "vip.detail", Display: "card", Group: "vip-zone"},
	}
	generated, ok := GenerateCompositePage("k", inputs, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("generate failed")
	}
	for _, sec := range generated.PageSpec.Composite.Sections {
		if sec.Display != "card" {
			t.Fatalf("display = %q, want card (section %s)", sec.Display, sec.Key)
		}
		if sec.Group != "vip-zone" {
			t.Fatalf("group = %q, want vip-zone (section %s)", sec.Group, sec.Key)
		}
		if sec.Key == "vip.detail" {
			if len(sec.CardTitle) != 0 {
				t.Fatalf("cardTitle should be absent, got %q (section %s)", sec.CardTitle["zh-CN"], sec.Key)
			}
			continue
		}
		if got := sec.CardTitle["zh-CN"]; got != "VIP 专区" {
			t.Fatalf("cardTitle = %q, want VIP 专区 (section %s)", got, sec.Key)
		}
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

// TestCompositeVisibleWhenPassthrough U10 区块级条件显示：visibleWhen
// 从 Input 透传到发布 spec（渲染端 sectionVisible 消费；漏传=条件失效）。
func TestCompositeVisibleWhenPassthrough(t *testing.T) {
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
	cond := &spec.ConditionSpec{
		Kind:  "equals",
		Key:   "filter-panel",
		Path:  "/values/mode",
		Value: json.RawMessage(`"advanced"`),
	}
	inputs := []CompositeSectionInput{
		{FunctionID: "player.list", View: "table", VisibleWhen: cond},
		{FunctionID: "player.list", View: "table", Key: "vip.rank"},
	}
	generated, ok := GenerateCompositePage("k", inputs, contracts, DefaultGenerateOptions())
	if !ok {
		t.Fatal("generate failed")
	}
	byKey := map[string]spec.CompositeSection{}
	for _, sec := range generated.PageSpec.Composite.Sections {
		byKey[sec.Key] = sec
	}
	got := byKey["player.list"].VisibleWhen
	if got == nil {
		t.Fatal("visibleWhen lost")
	}
	if got.Kind != "equals" || got.Key != "filter-panel" || got.Path != "/values/mode" || string(got.Value) != `"advanced"` {
		t.Fatalf("visibleWhen wrong: %+v", got)
	}
	if byKey["vip.rank"].VisibleWhen != nil {
		t.Fatalf("unconditional section should not carry visibleWhen: %+v", byKey["vip.rank"].VisibleWhen)
	}
}
