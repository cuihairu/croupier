package gate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/source"
)

// rowJSON 行像构造（C1 现实：位置数组 JSON）。
func rowJSON(vals ...interface{}) json.RawMessage {
	b, err := json.Marshal(vals)
	if err != nil {
		panic(err) // 仅基础类型入参，构造不可失败
	}
	return b
}

// itemEvent 道具表事件：行像 [ "junk", key ]，键规格 "1"。
func itemEvent(op source.Op, key string, tsMs int64) source.ChangeEvent {
	var before, after json.RawMessage
	img := rowJSON("junk", key)
	if op == source.OpDelete {
		before = img
	} else {
		after = img
	}
	return source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "item", Op: op,
		BeforeJSON: before, AfterJSON: after,
		GTID: "gtid-item-" + key + "-" + string(op), TsUnixMs: tsMs,
	}
}

// ledgerEvent 流水表事件：行像 [ key, "memo" ]，键规格 "0"。
func ledgerEvent(key string, tsMs int64) source.ChangeEvent {
	return source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "ledger", Op: source.OpInsert,
		AfterJSON: rowJSON(key, "memo"),
		GTID:      "gtid-ledger-" + key, TsUnixMs: tsMs,
	}
}

func testRule() ReconcileRule {
	return ReconcileRule{
		Name: "item-ledger", ItemTable: "game.item", LedgerTable: "game.ledger",
		ItemKey: "1", LedgerKey: "0", WindowSec: 60,
	}
}

func newGate(t *testing.T, rules ...ReconcileRule) (*ReconcileGate, *int64) {
	t.Helper()
	g, err := NewReconcile(rules)
	if err != nil {
		t.Fatalf("NewReconcile: %v", err)
	}
	now := int64(1_000_000)
	g.SetNow(func() int64 { return now })
	return g, &now
}

func TestReconcileItemThenLedgerMatches(t *testing.T) {
	g, _ := newGate(t, testRule())
	if hits := g.Evaluate(mustPtr(itemEvent(source.OpInsert, "100", 999_000))); len(hits) != 0 {
		t.Fatalf("item enqueue should not hit: %+v", hits)
	}
	if g.PendingCount() != 1 {
		t.Fatalf("pending = %d, want 1", g.PendingCount())
	}
	if hits := g.Evaluate(mustPtr(ledgerEvent("100", 999_500))); len(hits) != 0 {
		t.Fatalf("ledger match should not hit: %+v", hits)
	}
	if g.PendingCount() != 0 {
		t.Fatalf("pending after match = %d, want 0", g.PendingCount())
	}
	// 超窗扫一遍：无残留命中
	if hits := g.Sweep(1_000_000 + 61_000); len(hits) != 0 {
		t.Fatalf("residual hits: %+v", hits)
	}
}

func TestReconcileLedgerFirstCreditConsumed(t *testing.T) {
	g, _ := newGate(t, testRule())
	if hits := g.Evaluate(mustPtr(ledgerEvent("200", 999_000))); len(hits) != 0 {
		t.Fatalf("credit enqueue should not hit: %+v", hits)
	}
	if g.PendingCount() != 1 {
		t.Fatalf("pending = %d, want 1", g.PendingCount())
	}
	if hits := g.Evaluate(mustPtr(itemEvent(source.OpUpdate, "200", 999_500))); len(hits) != 0 {
		t.Fatalf("item consuming credit should not hit: %+v", hits)
	}
	if g.PendingCount() != 0 {
		t.Fatalf("pending = %d, want 0", g.PendingCount())
	}
}

func TestReconcileMissingLedgerSweepHit(t *testing.T) {
	g, now := newGate(t, testRule())
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "300", *now)))
	*now += 61_000
	hits := g.Sweep(*now)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1: %+v", len(hits), hits)
	}
	h := hits[0]
	if h.Rule != RuleReconcileMissingLedger {
		t.Errorf("rule = %q", h.Rule)
	}
	if h.Severity != SeverityWarn {
		t.Errorf("default severity = %q, want warn", h.Severity)
	}
	if h.Table != "game.item" || h.Key != "300" {
		t.Errorf("table/key = %q/%q, want game.item/300", h.Table, h.Key)
	}
	if h.Event.GTID != "gtid-item-300-insert" {
		t.Errorf("event snapshot gtid = %q", h.Event.GTID)
	}
	if g.PendingCount() != 0 {
		t.Errorf("pending after sweep = %d, want 0", g.PendingCount())
	}
}

func TestReconcileWithinWindowNoHit(t *testing.T) {
	g, now := newGate(t, testRule())
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "400", *now)))
	// 恰好 window-1ms：不命中
	hits := g.Sweep(*now + 59_999)
	if len(hits) != 0 {
		t.Fatalf("hits before window: %+v", hits)
	}
	// 恰好 window：命中
	if hits := g.Sweep(*now + 60_000); len(hits) != 1 {
		t.Fatalf("hits at window = %d, want 1", len(hits))
	}
}

func TestReconcileExemptKeySkipped(t *testing.T) {
	rule := testRule()
	rule.ExemptKeys = []string{"777"}
	g, now := newGate(t, rule)
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "777", *now)))
	if g.PendingCount() != 0 {
		t.Fatalf("exempt item should not enqueue, pending = %d", g.PendingCount())
	}
	*now += 61_000
	if hits := g.Sweep(*now); len(hits) != 0 {
		t.Fatalf("exempt item hit: %+v", hits)
	}
}

func TestReconcileMissingKeySentinelAlert(t *testing.T) {
	g, now := newGate(t, testRule())
	// 键规格越界：道具侧取不到键 → 哨兵入窗（宁误报不漏报）
	ev := itemEvent(source.OpInsert, "500", *now)
	ev.AfterJSON = rowJSON("only-one-column")
	g.Evaluate(mustPtr(ev))
	if g.PendingCount() != 1 {
		t.Fatalf("sentinel should enqueue, pending = %d", g.PendingCount())
	}
	// 流水事件永远无法匹配哨兵键（真实键非空）
	g.Evaluate(mustPtr(ledgerEvent("500", *now)))
	if g.PendingCount() != 2 {
		t.Fatalf("ledger should not consume sentinel, pending = %d", g.PendingCount())
	}
	*now += 61_000
	hits := g.Sweep(*now)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1 (sentinel only): %+v", len(hits), hits)
	}
	if hits[0].Key != "" {
		t.Errorf("sentinel hit key = %q, want empty", hits[0].Key)
	}
}

func TestReconcileLedgerNoKeySkipped(t *testing.T) {
	g, _ := newGate(t, testRule())
	// 流水行像空数组：键规格 "0" 越界 → 跳过并计数（不挂窗）
	ev := ledgerEvent("600", 999_000)
	ev.AfterJSON = rowJSON()
	g.Evaluate(mustPtr(ev))
	if g.SkippedNoKey() != 1 {
		t.Fatalf("SkippedNoKey = %d, want 1", g.SkippedNoKey())
	}
	if g.PendingCount() != 0 {
		t.Fatalf("skipped ledger should not enqueue, pending = %d", g.PendingCount())
	}
}

func TestReconcileDeleteUsesBeforeImage(t *testing.T) {
	g, _ := newGate(t, testRule())
	g.Evaluate(mustPtr(itemEvent(source.OpDelete, "700", 999_000)))
	if g.PendingCount() != 1 {
		t.Fatalf("delete should extract from before image, pending = %d", g.PendingCount())
	}
	g.Evaluate(mustPtr(ledgerEvent("700", 999_500)))
	if g.PendingCount() != 0 {
		t.Fatalf("delete key should match ledger, pending = %d", g.PendingCount())
	}
}

func TestReconcileUpdateUsesAfterImage(t *testing.T) {
	g, _ := newGate(t, testRule())
	// update：前像键 800-old，后像键 800；规格取后像 → 与 800 流水匹配
	ev := itemEvent(source.OpUpdate, "800", 999_000)
	ev.BeforeJSON = rowJSON("junk", "800-old")
	g.Evaluate(mustPtr(ev))
	g.Evaluate(mustPtr(ledgerEvent("800", 999_500)))
	if g.PendingCount() != 0 {
		t.Fatalf("update after-image key should match, pending = %d", g.PendingCount())
	}
}

func TestReconcileReplayBackfillNoInstantExpire(t *testing.T) {
	g, now := newGate(t, testRule())
	// 事件 ts 远早于窗口（binlog 回放补扫）：入窗时刻取 max(事件ts, now)
	old := *now - 10_000_000
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "900", old)))
	if hits := g.Sweep(*now + 1); len(hits) != 0 {
		t.Fatalf("backfilled entry must get fresh window: %+v", hits)
	}
	*now += 61_000
	if hits := g.Sweep(*now); len(hits) != 1 {
		t.Fatalf("hits after fresh window = %d, want 1", len(hits))
	}
}

func TestReconcileCapOverflowEviction(t *testing.T) {
	rule := testRule()
	rule.MaxPending = 2
	g, _ := newGate(t, rule)
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "a", 999_000)))
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "b", 999_001)))
	hits := g.Evaluate(mustPtr(itemEvent(source.OpInsert, "c", 999_002)))
	if len(hits) != 1 {
		t.Fatalf("cap eviction hits = %d, want 1", len(hits))
	}
	if !strings.Contains(hits[0].Detail, "cap 2 exceeded") {
		t.Errorf("eviction detail = %q", hits[0].Detail)
	}
	if hits[0].Key != "a" {
		t.Errorf("evicted key = %q, want a (oldest)", hits[0].Key)
	}
	if g.PendingCount() != 2 {
		t.Errorf("pending = %d, want 2", g.PendingCount())
	}
	// 被驱逐的 a 不再产 second hit；b/c 超窗各命中一次
	hits = g.Sweep(1_000_000 + 61_000)
	if len(hits) != 2 {
		t.Fatalf("sweep hits = %d, want 2: %+v", len(hits), hits)
	}
}

func TestReconcileCreditEvictedSilently(t *testing.T) {
	rule := testRule()
	rule.MaxPending = 1
	g, _ := newGate(t, rule)
	// credit 挂满容量（键 y），不同键的 debit 入窗触发驱逐：credit 静默清
	g.Evaluate(mustPtr(ledgerEvent("y", 999_000)))
	if hits := g.Evaluate(mustPtr(itemEvent(source.OpInsert, "x", 999_001))); len(hits) != 0 {
		t.Fatalf("credit eviction must be silent: %+v", hits)
	}
	if g.PendingCount() != 1 {
		t.Fatalf("pending = %d, want 1 (only the debit)", g.PendingCount())
	}
}

func TestReconcileTwoRulesIndependent(t *testing.T) {
	ruleB := ReconcileRule{
		Name: "mail-ledger", ItemTable: "game.mail", LedgerTable: "game.mail_log",
		ItemKey: "0", LedgerKey: "1", WindowSec: 30, Severity: SeverityCritical,
	}
	g, now := newGate(t, testRule(), ruleB)
	// 规则 A 道具入窗；规则 B 流水先到（同键值也不跨规则匹配）
	g.Evaluate(mustPtr(itemEvent(source.OpInsert, "100", *now)))
	g.Evaluate(mustPtr(source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "mail_log", Op: source.OpInsert,
		AfterJSON: rowJSON("memo", "100"), GTID: "gtid-b", TsUnixMs: *now,
	}))
	if g.PendingCount() != 2 {
		t.Fatalf("cross-rule match detected, pending = %d, want 2", g.PendingCount())
	}
	// A 超窗命中（warn）；B 的 credit 同刻静默清
	*now += 61_000
	hits := g.Sweep(*now)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1: %+v", len(hits), hits)
	}
	if hits[0].Severity != SeverityWarn || hits[0].Table != "game.item" {
		t.Errorf("hit = %+v", hits[0])
	}
}

func TestReconcileHitOrderDeterministic(t *testing.T) {
	g, now := newGate(t, testRule())
	for _, k := range []string{"b", "a", "c"} {
		g.Evaluate(mustPtr(itemEvent(source.OpInsert, k, *now)))
	}
	hits := g.Sweep(*now + 61_000)
	if len(hits) != 3 {
		t.Fatalf("hits = %d, want 3", len(hits))
	}
	for i, want := range []string{"a", "b", "c"} {
		if hits[i].Key != want {
			t.Errorf("hits[%d].Key = %q, want %q", i, hits[i].Key, want)
		}
	}
}

func TestReconcileValidation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ReconcileRule)
		wantErr string
	}{
		{"ok", func(*ReconcileRule) {}, ""},
		{"no name", func(r *ReconcileRule) { r.Name = "" }, "name is required"},
		{"bad item table", func(r *ReconcileRule) { r.ItemTable = "noseparator" }, "itemTable"},
		{"bad ledger table", func(r *ReconcileRule) { r.LedgerTable = "" }, "ledgerTable"},
		{"same tables", func(r *ReconcileRule) { r.LedgerTable = r.ItemTable }, "must differ"},
		{"no item key", func(r *ReconcileRule) { r.ItemKey = "" }, "itemKey/ledgerKey"},
		{"negative window", func(r *ReconcileRule) { r.WindowSec = -1 }, "windowSec"},
		{"bad severity", func(r *ReconcileRule) { r.Severity = "info" }, "severity"},
		{"negative cap", func(r *ReconcileRule) { r.MaxPending = -5 }, "maxPending"},
	}
	for _, tc := range cases {
		rule := testRule()
		tc.mutate(&rule)
		err := rule.Validate()
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error = %v, want contains %q", tc.name, err, tc.wantErr)
		}
	}

	// 同表跨规则复用：角色路由必须无歧义（r2 的道具表占用 r1 的流水表）
	dup := testRule()
	dup2 := testRule()
	dup2.Name = "r2"
	dup2.ItemTable = dup.LedgerTable
	dup2.LedgerTable = "game.mail_log"
	if err := ValidateReconcileRules([]ReconcileRule{dup, dup2}); err == nil ||
		!strings.Contains(err.Error(), "already used by rule") {
		t.Errorf("duplicate table error = %v", err)
	}
	if err := ValidateReconcileRules([]ReconcileRule{dup}); err != nil {
		t.Errorf("single rule error = %v", err)
	}
}

// mustPtr 供 Evaluate 取址。
func mustPtr(ev source.ChangeEvent) *source.ChangeEvent { return &ev }
