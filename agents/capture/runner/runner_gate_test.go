package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/gate"
	"github.com/cuihairu/croupier/agents/capture/source"
)

func gateRow(vals ...interface{}) json.RawMessage {
	b, err := json.Marshal(vals)
	if err != nil {
		panic(err) // 仅基础类型入参，构造不可失败
	}
	return b
}

func TestGateWiringEvaluateAndSweep(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		Name: "item-ledger", ItemTable: "game.items", LedgerTable: "game.ledger",
		ItemKey: "1", LedgerKey: "0", WindowSec: 60,
	}}
	r := New(cfg, nil)
	if r.reconcile == nil {
		t.Fatal("reconcile gate should be built from config rules")
	}

	// 道具变更入窗（位置数组行像，键在下标 1）
	item := source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Op: "insert",
		AfterJSON: gateRow("junk", "9001"), GTID: "g1", TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(item)
	if st := r.Stats(); st.GateHits != 0 {
		t.Fatalf("gate hits before expiry = %d, want 0", st.GateHits)
	}

	// 同键流水销账 → 不命中
	ledger := source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "ledger", Op: "insert",
		AfterJSON: gateRow("9001", "memo"), GTID: "g2", TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(ledger)
	if st := r.Stats(); st.GateHits != 0 {
		t.Fatalf("gate hits after ledger match = %d, want 0", st.GateHits)
	}
	if r.reconcile.PendingCount() != 0 {
		t.Fatalf("pending = %d, want 0", r.reconcile.PendingCount())
	}

	// 未销账道具超窗 → Sweep 产出命中并计数
	r.consume(item)
	r.sweepGates(time.Now().UnixMilli() + 61_000)
	if st := r.Stats(); st.GateHits != 1 {
		t.Fatalf("gate hits after sweep = %d, want 1", st.GateHits)
	}
}

func TestGateNotWiredWithoutRules(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if r.reconcile != nil {
		t.Fatal("gate must stay nil without rules")
	}
	r.consume(source.ChangeEvent{Op: "insert"})
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.GateHits != 0 || st.Total != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestConfigValidateRejectsBadGateRules(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		Name: "bad", ItemTable: "game.items", LedgerTable: "game.items",
		ItemKey: "1", LedgerKey: "0",
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("same item/ledger table must fail validation")
	}
}
