package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeRecorder 用 channel 同步生命周期事件（避免 sleep 读共享字段的 data race）。
type fakeRecorder struct {
	started chan struct{}
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{started: make(chan struct{}, 1)}
}

func (f *fakeRecorder) Start(context.Context, map[string]string) {
	select {
	case f.started <- struct{}{}:
	default:
	}
}

func (f *fakeRecorder) Stop(context.Context, error)         {}
func (f *fakeRecorder) Connected(context.Context)           {}
func (f *fakeRecorder) Disconnected(context.Context, error) {}
func (f *fakeRecorder) Count(context.Context, string, int64, map[string]string) {
}
func (f *fakeRecorder) Observe(context.Context, string, float64, map[string]string) {}

// SetRecorder 装配自定义 recorder 后 Run 应触发 Start；ctx 取消后 Run 返回
// （devops.enabled=false 时 Run=注册上线+等 ctx，退出路径最短）。
func TestSetRecorder_LifecycleStartObserved(t *testing.T) {
	r := New(testConfig(false), nil)
	rec := newFakeRecorder()
	r.SetRecorder(rec)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	select {
	case <-rec.started:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("expected Start recorded")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("expected Run to return after cancel")
	}
}

// SetRecorder(nil) 回退 noop（默认关闭=零开销路径契约）。
func TestSetRecorder_NilFallsBackToNoop(t *testing.T) {
	r := New(testConfig(false), nil)
	r.SetRecorder(nil)
	require.NotNil(t, r.rec)
}
