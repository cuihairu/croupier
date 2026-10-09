package outlet

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type stubOutlet struct {
	name  string
	calls atomic.Int64

	fn func(ctx context.Context, ev AlertEvent) error
}

func (s *stubOutlet) Name() string { return s.name }

func (s *stubOutlet) Deliver(ctx context.Context, ev AlertEvent) error {
	s.calls.Add(1)
	return s.fn(ctx, ev)
}

func sampleEvent() AlertEvent {
	return AlertEvent{
		Kind:             KindSupervisorBreaker,
		Severity:         SeverityCritical,
		EventID:          "agent-1:42",
		DedupKey:         "supervisor.breaker_tripped:agent-1:42",
		Title:            "breaker tripped",
		Body:             "too many restart failures",
		OccurredAtUnixMs: 1760000000000,
		Scope:            Scope{AgentID: "agent-1"},
	}
}

func TestManagerRegisterAndFanOut(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {} // 测试不真睡
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) error { return nil }}
	b := &stubOutlet{name: "webhook", fn: func(context.Context, AlertEvent) error { return nil }}
	m.Register(a)
	m.Register(nil) // nil 忽略
	m.Register(b)

	if got := m.Names(); len(got) != 2 || got[0] != "herald" || got[1] != "webhook" {
		t.Fatalf("Names() = %v, want [herald webhook]", got)
	}

	m.Dispatch(sampleEvent())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (a.calls.Load() == 0 || b.calls.Load() == 0) {
		time.Sleep(5 * time.Millisecond)
	}
	if a.calls.Load() != 1 || b.calls.Load() != 1 {
		t.Fatalf("fan-out calls = a:%d b:%d, want 1/1", a.calls.Load(), b.calls.Load())
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0", m.Failures())
	}
}

func TestManagerRetriesTransportThenSucceeds(t *testing.T) {
	m := NewManager()
	retries := 0
	m.sleep = func(d time.Duration) {
		if d != dispatchRetryDelay {
			t.Errorf("sleep(%v), want %v", d, dispatchRetryDelay)
		}
		retries++
	}
	o := &stubOutlet{name: "herald"}
	o.fn = func(context.Context, AlertEvent) error {
		if o.calls.Load() < 3 {
			return errors.New("dial tcp: connection refused") // transport 类：可重试
		}
		return nil
	}
	m.Register(o)

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && o.calls.Load() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	if o.calls.Load() != 3 {
		t.Fatalf("Deliver calls = %d, want 3 (two retries)", o.calls.Load())
	}
	if retries != 2 {
		t.Fatalf("sleep calls = %d, want 2", retries)
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0 after eventual success", m.Failures())
	}
}

func TestManagerStopsRetryOnPermanent(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) { t.Error("permanent error must not retry") }
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) error {
		return Permanent(errors.New("herald: 422: category not registered"))
	}}
	m.Register(o)

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && o.calls.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if o.calls.Load() != 1 {
		t.Fatalf("Deliver calls = %d, want 1 (no retry on permanent)", o.calls.Load())
	}
	if m.Failures() != 1 {
		t.Fatalf("Failures() = %d, want 1", m.Failures())
	}
}

func TestManagerExhaustsRetriesAndCounts(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) error {
		return errors.New("dial tcp: timeout")
	}}
	m.Register(o)

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && o.calls.Load() < dispatchAttempts {
		time.Sleep(5 * time.Millisecond)
	}
	if o.calls.Load() != dispatchAttempts {
		t.Fatalf("Deliver calls = %d, want %d", o.calls.Load(), dispatchAttempts)
	}
	if m.Failures() != 1 {
		t.Fatalf("Failures() = %d, want 1", m.Failures())
	}
}

func TestManagerRecoversFromOutletPanic(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) error {
		panic("outlet exploded")
	}}
	m.Register(o)

	m.Dispatch(sampleEvent()) // panic 被 recover，不传染
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && o.calls.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if o.calls.Load() != 1 {
		t.Fatalf("Deliver calls = %d, want 1", o.calls.Load())
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0 (panic happens before failure count)", m.Failures())
	}
}

func TestPermanentWrapping(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil")
	}
	wrapped := Permanent(errors.New("refused"))
	if !isPermanent(wrapped) {
		t.Fatal("wrapped error must report permanent")
	}
	if isPermanent(errors.New("plain")) {
		t.Fatal("plain error must not report permanent")
	}
	if unwrapped := errors.Unwrap(wrapped); unwrapped == nil || unwrapped.Error() != "refused" {
		t.Fatalf("Unwrap = %v, want refused", unwrapped)
	}
}

func TestDispatchWithoutOutlets(t *testing.T) {
	m := NewManager()
	m.Dispatch(sampleEvent()) // 无出口：空扇出不 panic
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0", m.Failures())
	}
}
