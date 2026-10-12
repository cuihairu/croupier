package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
)

// reconcileRulesFor 带对账规则 + 事件白名单的门面 JSON（验证过滤在进闸前生效）。
func reconcileRulesFor() string {
	return `{"reconcile":{"rules":[{"name":"item-ledger","itemTable":"game.items","ledgerTable":"game.ledger","itemKey":"1","ledgerKey":"0","windowSec":60}]},"event":{"ops":["insert","delete"]}}`
}

func TestEventFilterWiringFromConfig(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	if r.gates.Load().event != nil {
		t.Fatal("no event section = no filter (passthrough, backward compatible)")
	}

	if err := r.applyGates(context.Background(), []byte(reconcileRulesFor())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	gs := r.gates.Load()
	if gs.event == nil {
		t.Fatal("event section should wire an EventFilter")
	}
	if gs.reconcile == nil {
		t.Fatal("reconcile gate should still be wired alongside event filter")
	}

	// update 未列入白名单：进闸前丢弃，对账闸不收窗（FilteredNoMatch 计数）
	update := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpUpdate,
		After: contractRow("junk", "9001"), Position: connector.Position{Kind: "gtid", Value: "g1"},
		TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(update)
	if st := r.Stats(); st.FilteredNoMatch != 1 || st.GateHits != 0 {
		t.Fatalf("stats after filtered update = %+v, want FilteredNoMatch=1 GateHits=0", st)
	}
	if r.gates.Load().reconcile.PendingCount() != 0 {
		t.Fatalf("filtered update must not enter reconcile window, pending = %d", r.gates.Load().reconcile.PendingCount())
	}

	// insert 在白名单内：正常进闸入窗
	insert := update
	insert.Operation = connector.OpInsert
	r.consume(insert)
	if st := r.Stats(); st.FilteredNoMatch != 1 {
		t.Fatalf("listed op should pass filter, FilteredNoMatch = %d", st.FilteredNoMatch)
	}
	if r.gates.Load().reconcile.PendingCount() != 1 {
		t.Fatalf("listed op should enter reconcile window, pending = %d", r.gates.Load().reconcile.PendingCount())
	}
}

func TestEventFilterUnknownOpCountsError(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	if err := r.applyGates(context.Background(), []byte(reconcileRulesFor())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	weird := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: "truncate",
		Position: connector.Position{Kind: "gtid", Value: "g2"}, TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(weird)
	st := r.Stats()
	if st.FilteredError != 1 {
		t.Fatalf("FilteredError = %d, want 1 (observable drop, not silent)", st.FilteredError)
	}
	if st.Skipped != 1 {
		t.Fatalf("Skipped (op classification) = %d, want 1", st.Skipped)
	}
}

func TestEventFilterInvalidSectionKeepsLastGood(t *testing.T) {
	cfg := testConfig(t, true)
	r := New(cfg, nil)
	if err := r.applyGates(context.Background(), []byte(reconcileRulesFor())); err != nil {
		t.Fatalf("applyGates: %v", err)
	}
	// 非法 op：整体拒绝（all-or-nothing），keep last-good
	if err := r.applyGates(context.Background(), []byte(`{"event":{"ops":["bogus"]}}`)); err == nil {
		t.Fatal("invalid event ops should reject the whole gates payload")
	}
	gs := r.gates.Load()
	if gs.event == nil {
		t.Fatal("keep last-good: previous event filter should survive")
	}
	if gs.reconcile == nil {
		t.Fatal("keep last-good: previous reconcile gate should survive")
	}
	if got := gs.event.Evaluate("update"); got.String() != "no_match" {
		t.Fatalf("surviving filter behavior changed: %v", got)
	}
	// 空白名单（缺省段）：回到全放行且无过滤实例
	if err := r.applyGates(context.Background(), []byte(reconcileRuleJSON("item-ledger", "game.items", "game.ledger"))); err != nil {
		t.Fatalf("applyGates without event section: %v", err)
	}
	if r.gates.Load().event != nil {
		t.Fatal("event section absent should clear the filter")
	}
}

func TestBuildGateSetRejectsUnknownEventOp(t *testing.T) {
	var g config.Gates
	if err := json.Unmarshal([]byte(`{"event":{"ops":["drop"]}}`), &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := buildGateSet(g); err == nil {
		t.Fatal("buildGateSet should reject ops outside closed set")
	}
}
