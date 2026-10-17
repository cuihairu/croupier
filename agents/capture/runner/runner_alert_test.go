package runner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// hitFixture 造一条对账命中（未销账道具超窗）。
func hitFixture(t *testing.T) *Runner {
	t.Helper()
	cfg := testConfig(t, true)
	cfg.Capture.Gates.Reconcile.Rules = []gate.ReconcileRule{{
		Name: "item-ledger", ItemTable: "game.items", LedgerTable: "game.ledger",
		ItemKey: "1", LedgerKey: "0", WindowSec: 60,
	}}
	r := New(cfg, nil)
	item := connector.ChangeEvent{
		SourceType: "mysql", Database: "game", Table: "items", Operation: connector.OpInsert,
		After: contractRow("junk", "9001"), Position: connector.Position{Kind: "gtid", Value: "g-hit"},
		TsUnixMs: time.Now().UnixMilli(),
	}
	r.consume(item)
	r.sweepGates(time.Now().UnixMilli() + 61_000)
	if st := r.Stats(); st.GateHits != 1 {
		t.Fatalf("gate hits = %d, want 1", st.GateHits)
	}
	return r
}

// recordingSend 捕获上报并回执。
type recordingSend struct {
	mu   sync.Mutex
	sent []*opsv1.CaptureRuleHitReport
	ack  *opsv1.CaptureRuleHitAck
	err  error
}

func (s *recordingSend) send(_ context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, rep)
	if s.err != nil {
		return nil, s.err
	}
	return s.ack, nil
}

func (s *recordingSend) sentCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func (s *recordingSend) waitReport(t *testing.T) *opsv1.CaptureRuleHitReport {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		if len(s.sent) > 0 {
			rep := s.sent[0]
			s.mu.Unlock()
			return rep
		}
		s.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no alert report sent within timeout")
	return nil
}

func TestHitSenderSendsReport(t *testing.T) {
	r := hitFixture(t)
	send := &recordingSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true, Duplicate: false}}
	r.sendAlert = send.send

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)

	rep := send.waitReport(t)
	if rep.AgentId != "capture-test" || rep.GameId != "demo" || rep.Env != "prod" {
		t.Fatalf("report identity = %s/%s/%s", rep.AgentId, rep.GameId, rep.Env)
	}
	if rep.Rule != "reconcile.missing_ledger" || rep.Table != "game.items" {
		t.Fatalf("report rule/table = %s/%s", rep.Rule, rep.Table)
	}
	if rep.Key != "9001" || rep.Gtid != "g-hit" || rep.EventOp != "insert" || rep.EventTsUnixMs == 0 {
		t.Fatalf("report payload = %+v", rep)
	}
	if rep.Severity != "warn" {
		t.Fatalf("report severity = %s", rep.Severity)
	}
	if st := r.Stats(); st.AlertDropped != 0 {
		t.Fatalf("dropped = %d, want 0", st.AlertDropped)
	}
}

func TestHitSenderFailureCountsDrop(t *testing.T) {
	r := hitFixture(t)
	r.hitRetryInterval = time.Millisecond
	send := &recordingSend{err: errors.New("dial timeout")}
	r.sendAlert = send.send

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st := r.Stats(); st.AlertDropped >= 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("AlertDropped not incremented after send failure")
}

func TestHitSenderRetriesAfterFailure(t *testing.T) {
	r := hitFixture(t)
	r.hitRetryInterval = time.Millisecond
	send := &recordingSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true}}
	var attempts atomic.Int32
	r.sendAlert = func(ctx context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("server unavailable")
		}
		return send.send(ctx, rep)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for {
		if send.sentCount() >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed alert was not retried")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if st := r.Stats(); st.AlertDropped != 0 {
		t.Fatalf("dropped = %d, want 0 after successful retry", st.AlertDropped)
	}
}

func TestHitSenderDuplicateAckAccepted(t *testing.T) {
	r := hitFixture(t)
	send := &recordingSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true, Duplicate: true}}
	r.sendAlert = send.send

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)

	send.waitReport(t)
	// duplicate 回执不算丢弃，也不算失败。
	time.Sleep(50 * time.Millisecond)
	if st := r.Stats(); st.AlertDropped != 0 {
		t.Fatalf("dropped = %d, want 0 for duplicate ack", st.AlertDropped)
	}
}

func TestHitSenderQueueFullDrops(t *testing.T) {
	r := hitFixture(t)
	// 替换为无缓冲队列且无人消费：recordHits 非阻塞 → 直接丢。
	r.hitCh = make(chan gate.Hit)
	r.recordHits([]gate.Hit{{Rule: "r", Table: "t", Event: connector.ChangeEvent{TsUnixMs: time.Now().UnixMilli()}}})
	st := r.Stats()
	if st.AlertDropped != 1 {
		t.Fatalf("dropped = %d after queue-full hit, want 1", st.AlertDropped)
	}
}
