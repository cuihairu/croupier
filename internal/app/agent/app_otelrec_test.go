package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/core/otelrec"
)

// fakeRecorder 记录事件（mutex 保护：fireOnConnected 可能来自任意 goroutine）。
type fakeRecorder struct {
	mu           sync.Mutex
	starts       int
	stops        int
	connected    int
	disconnected []error
	counts       []recEvent
}

type recEvent struct {
	event string
	n     int64
	attrs map[string]string
}

func (f *fakeRecorder) Start(context.Context, map[string]string) {
	f.mu.Lock()
	f.starts++
	f.mu.Unlock()
}
func (f *fakeRecorder) Stop(context.Context, error) {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
}
func (f *fakeRecorder) Connected(context.Context) {
	f.mu.Lock()
	f.connected++
	f.mu.Unlock()
}
func (f *fakeRecorder) Disconnected(_ context.Context, err error) {
	f.mu.Lock()
	f.disconnected = append(f.disconnected, err)
	f.mu.Unlock()
}
func (f *fakeRecorder) Count(_ context.Context, event string, n int64, attrs map[string]string) {
	f.mu.Lock()
	f.counts = append(f.counts, recEvent{event: event, n: n, attrs: attrs})
	f.mu.Unlock()
}
func (f *fakeRecorder) Observe(context.Context, string, float64, map[string]string) {}

// SetRecorder(nil) 回退 noop（默认关闭=零开销路径契约）。
func TestAppSetRecorderNilFallsBackToNoop(t *testing.T) {
	a := New("addr", "agent")
	a.SetRecorder(nil)
	if a.rec == nil {
		t.Fatal("nil recorder must fall back to a non-nil noop")
	}
}

// 缺省（未装配）recorder 为非 nil noop：任何调用点无需判空。
func TestAppRecorderDefaultsToNoop(t *testing.T) {
	a := New("addr", "agent")
	if a.rec == nil {
		t.Fatal("default recorder must be non-nil noop")
	}
}

// OnConnected 链：recorder.Connected 先触发，用户回调随后（不丢用户回调）。
func TestAppOnConnectedChainsRecorderAndUserCallback(t *testing.T) {
	a := New("addr", "agent")
	rec := &fakeRecorder{}
	a.SetRecorder(rec)
	userCalled := false
	a.OnConnected(func() { userCalled = true })

	a.upstream.fireOnConnected()

	rec.mu.Lock()
	got := rec.connected
	rec.mu.Unlock()
	if got != 1 {
		t.Fatalf("recorder Connected = %d, want 1", got)
	}
	if !userCalled {
		t.Fatal("user OnConnected callback must still run")
	}
}

// OnDisconnected 链：recorder.Disconnected(err) 先触发，用户回调随后。
// SetRecorder 已把链装到上游槽位，直接经该槽位触发（白盒）。
func TestAppOnDisconnectedChainsRecorderAndUserCallback(t *testing.T) {
	a := New("addr", "agent")
	rec := &fakeRecorder{}
	a.SetRecorder(rec)
	userErr := error(nil)
	a.OnDisconnected(func(err error) { userErr = err })

	wantErr := errors.New("conn reset")
	a.upstream.onDisconnected(wantErr)

	rec.mu.Lock()
	got := len(rec.disconnected)
	gotErr := error(nil)
	if got > 0 {
		gotErr = rec.disconnected[0]
	}
	rec.mu.Unlock()
	if got != 1 || !errors.Is(gotErr, wantErr) {
		t.Fatalf("recorder Disconnected = %v (%v), want %v", got, gotErr, wantErr)
	}
	if !errors.Is(userErr, wantErr) {
		t.Fatalf("user Disconnected callback got %v, want %v", userErr, wantErr)
	}
}

// 断言 fakeRecorder 实现 Recorder（契约编译期检查）。
var _ otelrec.Recorder = (*fakeRecorder)(nil)
