package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/gate"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

func TestHitSpoolAcksAfterUpload(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	hit := r.pendingHit()
	if err := r.hitSpool.enqueue(hit); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	send := &recordingSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true}}
	r.sendAlert = send.send
	r.sendHit(context.Background(), hit)
	waitFor(t, func() bool { return send.sentCount() == 1 })
	waitFor(t, func() bool { return r.spoolCount() == 0 })
	if _, err := os.Stat(r.hitSpool.path); !os.IsNotExist(err) {
		t.Fatalf("acked spool should be removed, stat err=%v", err)
	}
}

func TestHitSpoolDeduplicatesPendingReport(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	hit := r.pendingHit()
	if err := r.hitSpool.enqueue(hit); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := r.hitSpool.enqueue(hit); err != nil {
		t.Fatalf("enqueue duplicate: %v", err)
	}
	if r.spoolCount() != 1 {
		t.Fatalf("spool count = %d, want 1", r.spoolCount())
	}
}

func TestHitSpoolRetriesAfterRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")

	first := hitFixture(t)
	first.hitSpool = newAlertSpool(path)
	if err := first.hitSpool.enqueue(first.pendingHit()); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	second := hitFixture(t)
	second.hitSpool = newAlertSpool(path)
	second.hitCh = make(chan gate.Hit, 1)
	if second.spoolCount() != 1 {
		t.Fatalf("spool count = %d, want 1 after restart", second.spoolCount())
	}
	send := &recordingSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true}}
	second.sendAlert = send.send
	second.startHitSender(context.Background())
	waitFor(t, func() bool { return send.sentCount() == 1 })
	waitFor(t, func() bool { return second.spoolCount() == 0 })
}

func TestHitSenderKeepsFailedAlertForRestart(t *testing.T) {
	r := hitFixture(t)
	r.hitRetryInterval = time.Millisecond
	var attempts atomic.Int32
	r.sendAlert = func(context.Context, *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		return nil, errors.New("server unavailable")
	}
	r.hitCh = make(chan gate.Hit, 1)
	hit := r.pendingHit()
	r.hitCh <- hit
	r.startHitSender(context.Background())
	waitFor(t, func() bool { return r.Stats().AlertDropped >= 1 })
	waitFor(t, func() bool { return r.spoolCount() == 1 })
	if attempts.Load() != 0 && r.spoolCount() != 1 {
		t.Fatal("failed alert must remain pending")
	}
}

func hitFixtureWithoutSpool(t *testing.T) *Runner {
	t.Helper()
	r := hitFixture(t)
	r.hitSpool = nil
	return r
}

func (r *Runner) pendingHit() gate.Hit {
	return gate.Hit{
		Rule: "reconcile.missing_ledger", Table: "game.items", Key: "9001",
		Event: connector.ChangeEvent{
			Position: connector.Position{Kind: "gtid", Value: "g-hit"},
			TsUnixMs: time.Now().UnixMilli(),
		},
	}
}

func (r *Runner) spoolCount() int {
	r.hitSpool.mu.Lock()
	defer r.hitSpool.mu.Unlock()
	return len(r.hitSpool.pending)
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
