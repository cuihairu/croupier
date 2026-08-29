package generator

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// 组合页：两个资源 → 两个 view 块、binding ID 带资源前缀、view 内引用同步改写。
func TestGenerateCompositePage_TwoResources(t *testing.T) {
	inputs := []CompositeResourceInput{
		{ResourceKey: "player", Semantics: playerSemantics(), Contracts: playerContracts()},
		{ResourceKey: "order", Semantics: orderSemantics(), Contracts: orderContracts()},
	}
	gen, ok := GenerateCompositePage("composite--player-order", inputs, DefaultGenerateOptions())
	if !ok {
		t.Fatal("composite should generate")
	}
	if gen.Type != spec.PageTypeComposite {
		t.Fatalf("type = %s", gen.Type)
	}
	if len(gen.Composite.Resources) != 2 {
		t.Fatalf("blocks = %d", len(gen.Composite.Resources))
	}
	// binding 前缀
	seen := map[string]bool{}
	for _, b := range gen.Bindings {
		seen[b.ID] = true
		if !strings.HasPrefix(b.ID, "player.") && !strings.HasPrefix(b.ID, "order.") {
			t.Fatalf("binding %s lacks resource prefix", b.ID)
		}
	}
	if !seen["player.list"] || !seen["order.list"] {
		t.Fatalf("expected player.list & order.list, got %v", seen)
	}
	// view 内 action 引用改写（player.update → player.update）
	for _, b := range gen.Composite.Resources {
		for _, a := range b.View.ListView.RowActions {
			if a.BindingID != "" && !strings.HasPrefix(a.BindingID, b.ResourceKey+".") {
				t.Fatalf("block %s action %s not rewritten", b.ResourceKey, a.BindingID)
			}
		}
	}
}

// 单资源可用 + 单资源缺失 → 生成但带诊断（basic）。
func TestGenerateCompositePage_OneSkipped(t *testing.T) {
	inputs := []CompositeResourceInput{
		{ResourceKey: "player", Semantics: playerSemantics(), Contracts: playerContracts()},
		{ResourceKey: "broken", Semantics: &model.CapabilitySemantics{ResourceKey: "broken"}, Contracts: nil},
	}
	gen, ok := GenerateCompositePage("composite--mixed", inputs, DefaultGenerateOptions())
	if !ok {
		t.Fatal("should still generate with one good resource")
	}
	if len(gen.Composite.Resources) != 1 {
		t.Fatalf("blocks = %d", len(gen.Composite.Resources))
	}
	found := false
	for _, d := range gen.Diagnostics {
		if strings.Contains(d.Message, "broken") {
			found = true
		}
	}
	if !found {
		t.Fatal("skipped resource should be diagnosed")
	}
}

// 全部不可生成 → ok=false。
func TestGenerateCompositePage_AllInvalid(t *testing.T) {
	inputs := []CompositeResourceInput{
		{ResourceKey: "a", Semantics: &model.CapabilitySemantics{ResourceKey: "a"}},
		{ResourceKey: "b", Semantics: &model.CapabilitySemantics{ResourceKey: "b"}},
	}
	if _, ok := GenerateCompositePage("composite--none", inputs, DefaultGenerateOptions()); ok {
		t.Fatal("all-invalid should not generate")
	}
}

// ---- fixtures（参照 golden_test 构造）----

func gormID(id uint) gorm.Model {
	return gormModelWithID(id)
}

func playerSemantics() *model.CapabilitySemantics {
	return &model.CapabilitySemantics{
		ResourceKey:       "player",
		CollectionQueryID: 101,
		IdentityField:     "id",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
}

func playerContracts() []*model.FunctionContract {
	return []*model.FunctionContract{{
		Model:        gormID(101),
		FunctionID:   "player.list",
		ResourceKey:  "player",
		Capability:   dbenum.CapabilityCollectionQuery,
		Execution:    string(spec.FunctionExecutionSync),
		Enabled:      true,
		InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`),
	}}
}

func orderSemantics() *model.CapabilitySemantics {
	return &model.CapabilitySemantics{
		ResourceKey:       "order",
		CollectionQueryID: 301,
		IdentityField:     "id",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
}

func orderContracts() []*model.FunctionContract {
	return []*model.FunctionContract{{
		Model:        gormID(301),
		FunctionID:   "order.list",
		ResourceKey:  "order",
		Capability:   dbenum.CapabilityCollectionQuery,
		Execution:    string(spec.FunctionExecutionSync),
		Enabled:      true,
		InputSchema:  model.JSON(`{"type":"object","properties":{"page":{"type":"integer"}}}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},"total":{"type":"integer"}}}`),
	}}
}
