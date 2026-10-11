package config

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/gate"
)

// ValidateGates 全量校验真值单测：静态校验（Validate*）与构造期检查
// （New*：对账表路由唯一性）+ 事件粗过滤合法性三层都要拦。
// 与 agent 侧 buildGateSet 的等价性由 runner 包 pinning 测试锁定。

func validGates() Gates {
	var g Gates
	g.Reconcile.Rules = []gate.ReconcileRule{
		{Name: "item-ledger", ItemTable: "game.item", LedgerTable: "game.ledger",
			ItemKey: "1", LedgerKey: "0", WindowSec: 60},
	}
	g.CEP.OffHours = []gate.OffHoursRule{
		{Name: "wallet-night", Table: "game.wallet",
			Windows: []gate.OffHoursWindow{{StartMin: 120, EndMin: 240}}},
	}
	g.Lineage.Rules = []gate.LineageRule{
		{Name: "wallet-allow", Table: "game.wallet", Ops: []string{"update"},
			AllowAccounts: []string{"app"}},
	}
	g.Event.Ops = []string{"insert", "update"}
	return g
}

func TestValidateGates_Valid(t *testing.T) {
	if err := ValidateGates(validGates()); err != nil {
		t.Fatalf("valid gates rejected: %v", err)
	}
}

func TestValidateGates_ReconcileStaticInvalid(t *testing.T) {
	g := validGates()
	g.Reconcile.Rules[0].ItemTable = "not-a-qualified-table"
	err := ValidateGates(g)
	if err == nil || !strings.Contains(err.Error(), "itemTable must be") {
		t.Fatalf("want itemTable validation error, got %v", err)
	}
}

func TestValidateGates_ReconcileDuplicateRouting(t *testing.T) {
	// 静态校验放行、构造期检查拦截：两条规则路由同一道具表。
	g := validGates()
	g.Reconcile.Rules = append(g.Reconcile.Rules, g.Reconcile.Rules[0])
	g.Reconcile.Rules[1].Name = "item-ledger-2"
	err := ValidateGates(g)
	if err == nil || !strings.Contains(err.Error(), "already used by rule") {
		t.Fatalf("want duplicate routing error, got %v", err)
	}
}

func TestValidateGates_CEPInvalid(t *testing.T) {
	g := validGates()
	g.CEP.OffHours[0].Windows = nil
	err := ValidateGates(g)
	if err == nil || !strings.Contains(err.Error(), "windows") {
		t.Fatalf("want cep windows validation error, got %v", err)
	}
}

func TestValidateGates_LineageInvalid(t *testing.T) {
	g := validGates()
	g.Lineage.Rules[0].AllowAccounts = nil
	err := ValidateGates(g)
	if err == nil || !strings.Contains(err.Error(), "allowAccounts") {
		t.Fatalf("want allowAccounts validation error, got %v", err)
	}
}

func TestValidateGates_EventOpsInvalid(t *testing.T) {
	g := validGates()
	g.Event.Ops = []string{"truncate"}
	err := ValidateGates(g)
	if err == nil || !strings.Contains(err.Error(), "event filter") {
		t.Fatalf("want event filter error, got %v", err)
	}
}

func TestValidateGates_EmptyGatesValid(t *testing.T) {
	// 空规则集合法（全闸关闭态，与缺省行为一致）。
	if err := ValidateGates(Gates{}); err != nil {
		t.Fatalf("empty gates rejected: %v", err)
	}
}
