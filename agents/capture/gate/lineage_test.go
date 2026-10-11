package gate

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/source"
)

func lineageEvent(op source.Op, account string) source.ChangeEvent {
	return source.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Op: op,
		AfterJSON: rowJSON("junk", "k"), GTID: "g-lineage-" + account + string(op),
		Account: account, TsUnixMs: tsOf(0, 12, 0),
	}
}

func TestLineageViolationHits(t *testing.T) {
	g, err := NewLineage([]LineageRule{{
		Name: "items-accounts", Table: "game.items",
		AllowAccounts: []string{"app_writer"}, Severity: SeverityCritical,
	}})
	if err != nil {
		t.Fatalf("NewLineage: %v", err)
	}
	ev := lineageEvent(source.OpUpdate, "root")
	hits := g.Evaluate(&ev)
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	h := hits[0]
	if h.Rule != RuleLineageWhitelistViolation || h.Severity != SeverityCritical || h.Key != "root" {
		t.Errorf("hit = %+v", h)
	}
	if !strings.Contains(h.Detail, `account "root"`) {
		t.Errorf("detail = %q", h.Detail)
	}
}

func TestLineageAllowedAccountNoHit(t *testing.T) {
	g, _ := NewLineage([]LineageRule{{
		Name: "items-accounts", Table: "game.items", AllowAccounts: []string{"app_writer"},
	}})
	ev := lineageEvent(source.OpInsert, "app_writer")
	if hits := g.Evaluate(&ev); len(hits) != 0 {
		t.Fatalf("allowed account should not hit: %+v", hits)
	}
}

func TestLineageEmptyAccountDegraded(t *testing.T) {
	g, _ := NewLineage([]LineageRule{{
		Name: "items-accounts", Table: "game.items", AllowAccounts: []string{"app_writer"},
	}})
	// C1 binlog 无会话账号：跳过计数，不误报
	for i := 0; i < 3; i++ {
		ev := lineageEvent(source.OpUpdate, "")
		if hits := g.Evaluate(&ev); len(hits) != 0 {
			t.Fatalf("degraded event should not hit: %+v", hits)
		}
	}
	if g.Degraded() != 3 {
		t.Fatalf("Degraded = %d, want 3", g.Degraded())
	}
}

func TestLineageOpsFilterAndTableRoute(t *testing.T) {
	g, _ := NewLineage([]LineageRule{{
		Name: "items-del", Table: "game.items", Ops: []string{"delete"},
		AllowAccounts: []string{"app_writer"},
	}})
	// insert 不在 ops 名单：直通且不计数
	ev := lineageEvent(source.OpInsert, "root")
	if hits := g.Evaluate(&ev); len(hits) != 0 || g.Degraded() != 0 {
		t.Fatalf("filtered op should pass silently: hits=%+v degraded=%d", hits, g.Degraded())
	}
	// 非订阅表：直通
	ev2 := lineageEvent(source.OpDelete, "root")
	ev2.Table = "other"
	if hits := g.Evaluate(&ev2); len(hits) != 0 {
		t.Fatalf("other table should not hit: %+v", hits)
	}
	// delete 白名单外：命中
	ev3 := lineageEvent(source.OpDelete, "root")
	if hits := g.Evaluate(&ev3); len(hits) != 1 {
		t.Fatalf("delete by root should hit: %+v", hits)
	}
}

func TestLineageValidation(t *testing.T) {
	ok := LineageRule{Name: "x", Table: "db.t", AllowAccounts: []string{"app"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid rule: %v", err)
	}
	if err := (LineageRule{Name: "", Table: "db.t", AllowAccounts: []string{"app"}}).Validate(); err == nil {
		t.Error("no name must fail")
	}
	if err := (LineageRule{Name: "x", Table: "bad", AllowAccounts: []string{"app"}}).Validate(); err == nil {
		t.Error("bad table must fail")
	}
	if err := (LineageRule{Name: "x", Table: "db.t"}).Validate(); err == nil || !strings.Contains(err.Error(), "allowAccounts") {
		t.Errorf("empty allowAccounts error = %v", err)
	}
	if err := (LineageRule{Name: "x", Table: "db.t", Ops: []string{"truncate"}, AllowAccounts: []string{"app"}}).Validate(); err == nil {
		t.Error("bad op must fail")
	}
	if err := (LineageRule{Name: "x", Table: "db.t", AllowAccounts: []string{"app"}, Severity: "info"}).Validate(); err == nil {
		t.Error("bad severity must fail")
	}
	if err := ValidateLineageRules(nil); err != nil {
		t.Errorf("empty lineage config: %v", err)
	}
}
