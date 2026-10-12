package runner

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
)

// contractRow 契约行像构造（位置下标键 map，ColumnsMapped=false 语义）。
func contractRow(vals ...interface{}) map[string]json.RawMessage {
	b, err := json.Marshal(vals)
	if err != nil {
		panic(err) // 仅基础类型入参，构造不可失败
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		panic(err)
	}
	m := make(map[string]json.RawMessage, len(arr))
	for i, v := range arr {
		m[strconv.Itoa(i)] = v
	}
	return m
}

func TestGateWiringEvaluateAndSweep(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		Name: "item-ledger", ItemTable: "game.items", LedgerTable: "game.ledger",
		ItemKey: "1", LedgerKey: "0", WindowSec: 60,
	}}
	r := New(cfg, nil)
	if r.gates.Load().reconcile == nil {
		t.Fatal("reconcile gate should be built from config rules")
	}

	// 道具变更入窗（位置下标键行像，键在 "1"）
	item := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpInsert,
		After: contractRow("junk", "9001"), Position: connector.Position{Kind: "gtid", Value: "g1"},
		TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(item)
	if st := r.Stats(); st.GateHits != 0 {
		t.Fatalf("gate hits before expiry = %d, want 0", st.GateHits)
	}

	// 同键流水销账 → 不命中
	ledger := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "ledger", Operation: connector.OpInsert,
		After: contractRow("9001", "memo"), Position: connector.Position{Kind: "gtid", Value: "g2"},
		TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(ledger)
	if st := r.Stats(); st.GateHits != 0 {
		t.Fatalf("gate hits after ledger match = %d, want 0", st.GateHits)
	}
	if r.gates.Load().reconcile.PendingCount() != 0 {
		t.Fatalf("pending = %d, want 0", r.gates.Load().reconcile.PendingCount())
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
	if r.gates.Load().reconcile != nil {
		t.Fatal("gate must stay nil without rules")
	}
	r.consume(connector.ChangeEvent{Operation: connector.OpInsert})
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

func TestGateCEPWiringAndHit(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.Gates.CEP.Burst = []gate.BurstRule{{
		Name: "wallet-drain", Table: "game.items", DeltaField: "1",
		Threshold: 100, Mode: "single",
	}}
	r := New(cfg, nil)
	if r.gates.Load().cep == nil {
		t.Fatal("cep gate should be built from config rules")
	}
	// 大额单笔突变：事件到达即命中
	big := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpUpdate,
		Before: contractRow("junk", 100), After: contractRow("junk", 9000),
		Position: connector.Position{Kind: "gtid", Value: "g-big"}, TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(big)
	if st := r.Stats(); st.GateHits != 1 {
		t.Fatalf("gate hits = %d, want 1", st.GateHits)
	}
	// Sweep 只做清桶卫生，不产命中
	r.sweepGates(time.Now().UnixMilli() + 60_000)
	if st := r.Stats(); st.GateHits != 1 {
		t.Fatalf("gate hits after sweep = %d, want 1", st.GateHits)
	}
}

func TestGateCEPNotWiredWithoutRules(t *testing.T) {
	r := New(testConfig(t, true), nil)
	if r.gates.Load().cep != nil {
		t.Fatal("cep gate must stay nil without rules")
	}
	r.consume(connector.ChangeEvent{Operation: connector.OpInsert})
	r.sweepGates(time.Now().UnixMilli())
	if st := r.Stats(); st.GateHits != 0 || st.Total != 1 {
		t.Fatalf("stats = %+v", st)
	}
}
