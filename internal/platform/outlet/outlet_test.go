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

	fn func(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error)
}

func (s *stubOutlet) Name() string { return s.name }

func (s *stubOutlet) Deliver(ctx context.Context, ev AlertEvent) (DeliveryOutcome, error) {
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

func TestManagerRegisterAndNames(t *testing.T) {
	m := NewManager()
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "herald"}, nil
	}}
	b := &stubOutlet{name: "webhook", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "webhook"}, nil
	}}
	m.Register(a)
	m.Register(nil) // nil 忽略
	m.Register(b)

	if got := m.Names(); len(got) != 2 || got[0] != "herald" || got[1] != "webhook" {
		t.Fatalf("Names() = %v, want [herald webhook]", got)
	}
}

// 链式降级：首出口成功即停，后续出口不再尝试（§5.1 主出口成功=链终止）。
func TestManagerChainStopsOnFirstSuccess(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {} // 测试不真睡
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "herald"}, nil
	}}
	b := &stubOutlet{name: "webhook", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "webhook"}, nil
	}}
	m.Register(a)
	m.Register(b)

	m.Dispatch(sampleEvent())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && a.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if a.calls.Load() != 1 {
		t.Fatalf("first outlet calls = %d, want 1", a.calls.Load())
	}
	if b.calls.Load() != 0 {
		t.Fatalf("second outlet calls = %d, want 0 (chain stops on first success)", b.calls.Load())
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0", m.Failures())
	}
}

// 链式降级：首出口 Retryable 失败滑下一个（§5.1 主出口失败滑下一个）。
func TestManagerChainSlidesOnRetryableFailure(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {} // 测试不真睡
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: true},
			errors.New("dial tcp: connection refused")
	}}
	b := &stubOutlet{name: "webhook", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "webhook"}, nil
	}}
	m.Register(a)
	m.Register(b)

	m.Dispatch(sampleEvent())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && b.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if a.calls.Load() != dispatchAttempts {
		t.Fatalf("first outlet calls = %d, want %d (retries exhausted)", a.calls.Load(), dispatchAttempts)
	}
	if b.calls.Load() != 1 {
		t.Fatalf("second outlet calls = %d, want 1 (chain must slide)", b.calls.Load())
	}
	if m.Failures() != 1 {
		t.Fatalf("Failures() = %d, want 1 (failed outlet counted)", m.Failures())
	}
}

// 链式降级：Permanent 拒绝不滑（重试无意义，链终止——§5.1 仅 Retryable 滑下一个）。
func TestManagerChainStopsOnPermanent(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) { t.Error("permanent error must not retry") }
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: false},
			Permanent(errors.New("herald: 422: category not registered"))
	}}
	b := &stubOutlet{name: "webhook", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: true, Channel: "webhook"}, nil
	}}
	m.Register(a)
	m.Register(b)

	m.Dispatch(sampleEvent())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && a.calls.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if a.calls.Load() != 1 {
		t.Fatalf("first outlet calls = %d, want 1 (no retry on permanent)", a.calls.Load())
	}
	if b.calls.Load() != 0 {
		t.Fatalf("second outlet calls = %d, want 0 (permanent stops chain)", b.calls.Load())
	}
	if m.Failures() != 1 {
		t.Fatalf("Failures() = %d, want 1", m.Failures())
	}
}

// 站内出口不受降级影响：外部全败仍恒投递（§5.1 任何配置下站内通知都可达）。
func TestManagerInternalDeliveredDespiteExternalFailure(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {} // 测试不真睡
	sink := &recordingSink{}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))
	a := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: true},
			errors.New("dial tcp: connection refused")
	}}
	m.Register(a)

	m.Dispatch(sampleEvent())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(sink.notices) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if len(sink.notices) != 1 {
		t.Fatalf("station notices = %d, want 1 (internal must always deliver)", len(sink.notices))
	}
	if sink.notices[0].To != DefaultStationRecipient {
		t.Fatalf("notice.To = %q, want default recipient", sink.notices[0].To)
	}
	if m.Failures() != 1 {
		t.Fatalf("Failures() = %d, want 1 (external failure still counted)", m.Failures())
	}
}

// recordingSink 记录 StationNotice（测试专用）。
type recordingSink struct {
	notices []StationNotice
}

func (r *recordingSink) Create(_ context.Context, n StationNotice) error {
	r.notices = append(r.notices, n)
	return nil
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
	o.fn = func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		if o.calls.Load() < 3 {
			return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: true},
				errors.New("dial tcp: connection refused") // transport 类：可重试
		}
		return DeliveryOutcome{Delivered: true, Channel: "herald"}, nil
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
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: false},
			Permanent(errors.New("herald: 422: category not registered"))
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
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
		return DeliveryOutcome{Delivered: false, Channel: "herald", Retryable: true},
			errors.New("dial tcp: timeout")
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
	o := &stubOutlet{name: "herald", fn: func(context.Context, AlertEvent) (DeliveryOutcome, error) {
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
