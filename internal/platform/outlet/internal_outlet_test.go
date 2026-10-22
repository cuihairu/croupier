package outlet

import (
	"context"
	"errors"
	"testing"
)

// TestInternalOutletMapsEnvelope 信封→StationNotice 全套字段映射（§5.1
// 通知强化：level/type/source/ref/scope + 分级映射）。
func TestInternalOutletMapsEnvelope(t *testing.T) {
	sink := &recordingSink{}
	o := NewInternalOutlet(sink, DefaultStationRecipient)

	ev := AlertEvent{
		Kind:             KindIncidentReport,
		Severity:         SeverityWarning,
		EventID:          "report:week:2026-W41:qa",
		DedupKey:         "report:week:2026-W41:qa",
		Title:            "周报：质量类事故 3 起",
		Body:             "环比 +50%",
		OccurredAtUnixMs: 1760000000000,
		Scope:            Scope{GameID: "demo", Env: "prod"},
		Target:           "category-leader:qa",
	}
	outcome, err := o.Deliver(context.Background(), ev)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !outcome.Delivered || outcome.Channel != OutletInternal {
		t.Fatalf("outcome = %+v, want delivered internal", outcome)
	}
	if len(sink.notices) != 1 {
		t.Fatalf("notices = %d, want 1", len(sink.notices))
	}
	n := sink.notices[0]
	if n.To != "qa" {
		t.Fatalf("To = %q, want qa (category-leader: 前缀去壳直投 leader)", n.To)
	}
	if n.Type != "report" {
		t.Fatalf("Type = %q, want report", n.Type)
	}
	if n.Level != "warn" {
		t.Fatalf("Level = %q, want warn (severity 映射)", n.Level)
	}
	if n.Source != "incident_report" {
		t.Fatalf("Source = %q, want incident_report", n.Source)
	}
	if n.RefType != "report" || n.RefID != "report:week:2026-W41:qa" {
		t.Fatalf("ref = %s/%s, want report/report:week:2026-W41:qa", n.RefType, n.RefID)
	}
	if n.Title != ev.Title || n.Content != ev.Body {
		t.Fatalf("title/content = %q / %q", n.Title, n.Content)
	}
	if n.Scope["gameId"] != "demo" || n.Scope["env"] != "prod" {
		t.Fatalf("scope = %v, want gameId+env", n.Scope)
	}
	if _, ok := n.Scope["agentId"]; ok {
		t.Fatalf("scope must omit empty agentId: %v", n.Scope)
	}
}

// TestInternalOutletAgentEventSource agent 事件 Source=AgentID（supervisor
// 熔断经此落库，溯源到具体 agent）。
func TestInternalOutletAgentEventSource(t *testing.T) {
	sink := &recordingSink{}
	o := NewInternalOutlet(sink, DefaultStationRecipient)

	ev := sampleEvent() // supervisor.breaker_tripped + Scope.AgentID
	outcome, err := o.Deliver(context.Background(), ev)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !outcome.Delivered {
		t.Fatalf("outcome = %+v, want delivered", outcome)
	}
	n := sink.notices[0]
	if n.Type != "alert" {
		t.Fatalf("Type = %q, want alert", n.Type)
	}
	if n.Source != "agent-1" {
		t.Fatalf("Source = %q, want agent-1", n.Source)
	}
	if n.RefType != "event" || n.RefID != "agent-1:42" {
		t.Fatalf("ref = %s/%s, want event/agent-1:42", n.RefType, n.RefID)
	}
	if n.Level != "critical" {
		t.Fatalf("Level = %q, want critical", n.Level)
	}
	if n.To != DefaultStationRecipient {
		t.Fatalf("To = %q, want default recipient (no Target)", n.To)
	}
}

// TestInternalOutletSinkFailureRetryable 落库失败按可重试分类（DB 抖动
// 重试有意义；管理器据此重试）。
func TestInternalOutletSinkFailureRetryable(t *testing.T) {
	o := NewInternalOutlet(&failingSink{err: errors.New("database is locked")}, DefaultStationRecipient)
	outcome, err := o.Deliver(context.Background(), sampleEvent())
	if err == nil {
		t.Fatal("want sink error")
	}
	if outcome.Delivered {
		t.Fatalf("outcome = %+v, want undelivered", outcome)
	}
	if !outcome.Retryable {
		t.Fatalf("outcome = %+v, want retryable", outcome)
	}
}

// TestStationRecipientParsing Target 前缀解析闭集。
func TestStationRecipientParsing(t *testing.T) {
	cases := map[string]string{
		"":                   DefaultStationRecipient,
		"user:alice":         "alice",
		"category-leader:qa": "qa",
		"group:gm-ops":       DefaultStationRecipient, // 非直投前缀 → 默认组
		"user:":              DefaultStationRecipient, // 空前缀 → 默认组
	}
	for target, want := range cases {
		if got := stationRecipient(target, DefaultStationRecipient); got != want {
			t.Fatalf("stationRecipient(%q) = %q, want %q", target, got, want)
		}
	}
}

// TestNoopOutletSilentSuccess noop 回执成功（链终止于静默，零外发）。
func TestNoopOutletSilentSuccess(t *testing.T) {
	o := NewNoopOutlet()
	if o.Name() != OutletNoop {
		t.Fatalf("Name = %s, want noop", o.Name())
	}
	outcome, err := o.Deliver(context.Background(), sampleEvent())
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if !outcome.Delivered || outcome.Channel != OutletNoop {
		t.Fatalf("outcome = %+v, want delivered noop", outcome)
	}
}

type failingSink struct{ err error }

func (f *failingSink) Create(context.Context, StationNotice) error { return f.err }
