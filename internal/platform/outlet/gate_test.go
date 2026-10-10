package outlet

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingOutlet 记录收到的信封（断言 gate 标注到达外部出口用）。
type recordingOutlet struct {
	name string
	mu   sync.Mutex
	evs  []AlertEvent
}

func (r *recordingOutlet) Name() string { return r.name }

func (r *recordingOutlet) Deliver(_ context.Context, ev AlertEvent) (DeliveryOutcome, error) {
	r.mu.Lock()
	r.evs = append(r.evs, ev)
	r.mu.Unlock()
	return DeliveryOutcome{Delivered: true, Channel: r.name}, nil
}

func (r *recordingOutlet) events() []AlertEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]AlertEvent(nil), r.evs...)
}

// 抑制=整链静默（站内也静默）：一次评估覆盖链入口，出口零投递、
// 抑制不计入投递失败（抑制不是失败——计数与日志由 gate 侧负责）。
func TestGateSuppressesWholeChain(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	sink := &recordingSink{}
	ext := &recordingOutlet{name: "herald"}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))
	m.Register(ext)

	var calls atomic.Int64
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		calls.Add(1)
		return true, map[string]string{MetaMaintenanceSuppressed: "true"}
	})

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // 让「本应发生的」投递窗口过去

	if calls.Load() != 1 {
		t.Fatalf("gate calls = %d, want 1 (evaluated once at chain entry)", calls.Load())
	}
	if got := sink.snapshot(); len(got) != 0 {
		t.Fatalf("station notices = %d, want 0 (suppressed incl. internal)", len(got))
	}
	if got := ext.events(); len(got) != 0 {
		t.Fatalf("external deliveries = %d, want 0", len(got))
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0 (suppression is not a failure)", m.Failures())
	}
}

// fail-open 照报：source_unknown 标注到达站内正文尾与外部出口信封。
func TestGateUnknownAnnotatesAndDelivers(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	sink := &recordingSink{}
	ext := &recordingOutlet{name: "herald"}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))
	m.Register(ext)
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		return false, map[string]string{MetaSourceUnknown: "true"}
	})

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	notices := sink.snapshot()
	for time.Now().Before(deadline) && len(notices) == 0 {
		time.Sleep(5 * time.Millisecond)
		notices = sink.snapshot()
	}
	if len(notices) != 1 {
		t.Fatalf("station notices = %d, want 1 (fail-open must still deliver)", len(notices))
	}
	want := sampleEvent().Body + " [" + MetaSourceUnknown + "]"
	if notices[0].Content != want {
		t.Fatalf("notice.Content = %q, want %q", notices[0].Content, want)
	}
	evs := ext.events()
	if len(evs) != 1 {
		t.Fatalf("external deliveries = %d, want 1", len(evs))
	}
	if got := evs[0].Metadata[MetaSourceUnknown]; got != "true" {
		t.Fatalf("external Metadata[%s] = %q, want true", MetaSourceUnknown, got)
	}
}

// 无法定位服务器的事件不过 gate（设计 §1.2）：照投、零标注、gate 零调用。
func TestGateSkippedWithoutAgentID(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	sink := &recordingSink{}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))

	var calls atomic.Int64
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		calls.Add(1)
		return true, map[string]string{MetaMaintenanceSuppressed: "true"}
	})

	ev := sampleEvent()
	ev.Scope = Scope{} // 无服务器定位
	m.Dispatch(ev)
	deadline := time.Now().Add(2 * time.Second)
	notices := sink.snapshot()
	for time.Now().Before(deadline) && len(notices) == 0 {
		time.Sleep(5 * time.Millisecond)
		notices = sink.snapshot()
	}
	if len(notices) != 1 {
		t.Fatalf("station notices = %d, want 1 (ungated event must deliver)", len(notices))
	}
	if calls.Load() != 0 {
		t.Fatalf("gate calls = %d, want 0 (no server location → no gate)", calls.Load())
	}
	if got := notices[0].Content; got != sampleEvent().Body {
		t.Fatalf("notice.Content = %q, want unannotated body", got)
	}
}

// DispatchExternal 抑制：返回可解释原因，外部链零投递。
func TestDispatchExternalSuppressed(t *testing.T) {
	m := NewManager()
	ext := &recordingOutlet{name: "herald"}
	m.Register(ext)
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		return true, map[string]string{MetaMaintenanceSuppressed: "true"}
	})

	outcome, err := m.DispatchExternal(context.Background(), sampleEvent())
	if !errors.Is(err, ErrSuppressedByMaintenance) || outcome.Delivered {
		t.Fatalf("outcome = %+v err = %v, want ErrSuppressedByMaintenance", outcome, err)
	}
	if got := ext.events(); len(got) != 0 {
		t.Fatalf("external deliveries = %d, want 0", len(got))
	}
	if m.Failures() != 0 {
		t.Fatalf("Failures() = %d, want 0", m.Failures())
	}
}

// DispatchExternal 无定位事件不过 gate：v1 报表推送路径语义不变。
func TestDispatchExternalUnscopedEventBypassesGate(t *testing.T) {
	m := NewManager()
	ext := &recordingOutlet{name: "herald"}
	m.Register(ext)
	var calls atomic.Int64
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		calls.Add(1)
		return true, nil
	})

	ev := sampleEvent()
	ev.Scope = Scope{}
	outcome, err := m.DispatchExternal(context.Background(), ev)
	if err != nil || !outcome.Delivered || outcome.Channel != "herald" {
		t.Fatalf("outcome = %+v err = %v, want delivered", outcome, err)
	}
	if calls.Load() != 0 {
		t.Fatalf("gate calls = %d, want 0", calls.Load())
	}
}

// 未挂 gate 时语义零变化（默认直通；回归防回退）。
func TestManagerNoGateDeliversPlain(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	sink := &recordingSink{}
	ext := &recordingOutlet{name: "herald"}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))
	m.Register(ext)

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	notices := sink.snapshot()
	for time.Now().Before(deadline) && len(notices) == 0 {
		time.Sleep(5 * time.Millisecond)
		notices = sink.snapshot()
	}
	if len(notices) != 1 {
		t.Fatalf("station notices = %d, want 1", len(notices))
	}
	if got := notices[0].Content; got != sampleEvent().Body {
		t.Fatalf("notice.Content = %q, want unannotated body", got)
	}
	evs := ext.events()
	if len(evs) != 1 || len(evs[0].Metadata) != 0 {
		t.Fatalf("external events = %+v, want one event without metadata", evs)
	}
}

// 标注卸载（SetGate(nil)）回到直通。
func TestSetGateNilUnwires(t *testing.T) {
	m := NewManager()
	m.sleep = func(time.Duration) {}
	sink := &recordingSink{}
	m.Register(NewInternalOutlet(sink, DefaultStationRecipient))
	m.SetGate(func(context.Context, AlertEvent) (bool, map[string]string) {
		return true, nil
	})
	m.SetGate(nil)

	m.Dispatch(sampleEvent())
	deadline := time.Now().Add(2 * time.Second)
	notices := sink.snapshot()
	for time.Now().Before(deadline) && len(notices) == 0 {
		time.Sleep(5 * time.Millisecond)
		notices = sink.snapshot()
	}
	if len(notices) != 1 {
		t.Fatalf("station notices = %d, want 1 after unwire", len(notices))
	}
}

func TestMetadataSuffix(t *testing.T) {
	if got := metadataSuffix(nil); got != "" {
		t.Fatalf("suffix(nil) = %q, want empty", got)
	}
	if got := metadataSuffix(map[string]string{}); got != "" {
		t.Fatalf("suffix(empty) = %q, want empty", got)
	}
	if got := metadataSuffix(map[string]string{MetaSourceUnknown: "true"}); got != " [source_unknown]" {
		t.Fatalf("suffix = %q, want \" [source_unknown]\"", got)
	}
	got := metadataSuffix(map[string]string{
		MetaMaintenanceSuppressed: "true",
		MetaSourceUnknown:         "true",
	})
	if got != " [maintenance_suppressed source_unknown]" {
		t.Fatalf("suffix = %q, want sorted deterministic output", got)
	}
}
