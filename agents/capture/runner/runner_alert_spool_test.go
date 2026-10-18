package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
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

func TestHitSpoolRestartsInPersistedOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")
	first := hitFixture(t)
	first.hitSpool = newAlertSpool(path)
	early := first.pendingHit()
	early.Key = "early"
	late := first.pendingHit()
	late.Key = "late"
	if err := first.hitSpool.enqueue(early); err != nil {
		t.Fatalf("enqueue early: %v", err)
	}
	if err := first.hitSpool.enqueue(late); err != nil {
		t.Fatalf("enqueue late: %v", err)
	}

	second := hitFixture(t)
	second.hitSpool = newAlertSpool(path)
	send := &orderedSend{ack: &opsv1.CaptureRuleHitAck{Accepted: true}}
	second.sendAlert = send.send
	second.startHitSender(context.Background())
	waitFor(t, func() bool { return second.spoolCount() == 0 })
	if keys := send.keys(); !reflect.DeepEqual(keys, []string{"early", "late"}) {
		t.Fatalf("replay keys = %v, want [early late]", keys)
	}
	var raw struct {
		Entries []struct {
			Key string `json:"key"`
		} `json:"entries"`
	}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	if len(raw.Entries) != 0 {
		t.Fatalf("persisted entries = %+v, want empty", raw.Entries)
	}
}

type orderedSend struct {
	mu   sync.Mutex
	sent []string
	ack  *opsv1.CaptureRuleHitAck
}

func (s *orderedSend) send(_ context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, rep.GetKey())
	return s.ack, nil
}

func (s *orderedSend) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.sent...)
}

func TestAlertSpoolRejectsBeyondLimit(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	r.hitSpool.limit = 2

	for index := range 2 {
		hit := r.pendingHit()
		hit.Key = fmt.Sprintf("key-%d", index)
		hit.Event.TsUnixMs += int64(index)
		if err := r.hitSpool.enqueue(hit); err != nil {
			t.Fatalf("enqueue %d: %v", index, err)
		}
	}
	hit := r.pendingHit()
	hit.Key = "key-overflow"
	if err := r.hitSpool.enqueue(hit); err == nil {
		t.Fatal("enqueue beyond limit accepted")
	}
	if r.spoolCount() != 2 {
		t.Fatalf("spool count = %d, want 2", r.spoolCount())
	}
}

func TestAlertSpoolWriteFails(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	r.hitSpool.writeFile = func(string, []byte) error {
		return errors.New("disk unavailable")
	}
	if err := r.hitSpool.enqueue(r.pendingHit()); err == nil {
		t.Fatal("failed spool write accepted")
	}
}

func TestAlertSpoolAckWriteFailRollsBack(t *testing.T) {
	r := hitFixture(t)
	dir := t.TempDir()
	r.hitSpool = newAlertSpool(filepath.Join(dir, "alerts.json"))
	hit := r.pendingHit()
	if err := r.hitSpool.enqueue(hit); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	key := r.hitSpool.key(hit)
	r.hitSpool.writeFile = func(string, []byte) error {
		return errors.New("disk unavailable")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("make spool directory read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	if err := r.hitSpool.ack(key); err == nil {
		t.Fatal("failed spool ack write returned nil")
	}
	restored, ok := r.hitSpool.next()
	if !ok || !reflect.DeepEqual(restored, hit) {
		t.Fatalf("restored hit = %+v, ok=%v, want original pending hit", restored, ok)
	}
}

func TestRecordHitsPersistsWhenQueueSaturated(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	r.hitCh = make(chan gate.Hit, 1)
	r.hitRetryInterval = time.Hour

	first := r.pendingHit()
	first.Key = "queued"
	first.Event.TsUnixMs++
	second := r.pendingHit()
	second.Key = "spilled"
	second.Event.TsUnixMs += 2
	r.sendAlert = func(context.Context, *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		return nil, errors.New("server unavailable")
	}
	r.recordHits([]gate.Hit{first, second})

	if r.spoolCount() != 2 {
		t.Fatalf("spool count = %d, want 2", r.spoolCount())
	}
	if st := r.Stats(); st.AlertDropped != 0 {
		t.Fatalf("dropped = %d, want 0 when alert is durable", st.AlertDropped)
	}
}

func TestRecordHitsDropsWhenSpoolWriteFails(t *testing.T) {
	r := hitFixture(t)
	r.hitSpool = newAlertSpool(r.cfg.AlertSpoolPath())
	r.hitSpool.writeFile = func(string, []byte) error {
		return errors.New("disk unavailable")
	}
	r.hitCh = make(chan gate.Hit)
	r.sendAlert = func(context.Context, *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		return nil, errors.New("server unavailable")
	}

	r.recordHits([]gate.Hit{r.pendingHit()})
	if st := r.Stats(); st.AlertDropped != 1 {
		t.Fatalf("dropped = %d, want 1 when spool write fails", st.AlertDropped)
	}
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

func TestHitSenderRetriesDurableAlertAfterExhaustingAttempts(t *testing.T) {
	r := hitFixture(t)
	r.hitRetryInterval = time.Millisecond
	send := &recordingSend{err: errors.New("server unavailable")}
	r.sendAlert = send.send
	hit := r.pendingHit()
	if err := r.hitSpool.enqueue(hit); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.startHitSender(ctx)
	waitFor(t, func() bool { return send.sentCount() > hitMaxAttempts })
	if r.spoolCount() != 1 {
		t.Fatalf("spool count = %d, want 1 while retrying", r.spoolCount())
	}
}

func hitFixtureWithoutSpool(t *testing.T) *Runner {
	t.Helper()
	r := hitFixture(t)
	r.hitSpool = nil
	return r
}

func clearHitSpool(t *testing.T, r *Runner) {
	t.Helper()
	for len(r.hitSpool.order) > 0 {
		key := r.hitSpool.order[0]
		if err := r.hitSpool.ack(key); err != nil {
			t.Fatalf("clear spool: %v", err)
		}
	}
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
