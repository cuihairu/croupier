package runner

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/stretchr/testify/require"
)

// fakeRecorder 用 channel 同步生命周期事件（避免 sleep 读共享字段的 data race）。
type fakeRecorder struct {
	started chan struct{}
	stopped chan error
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{started: make(chan struct{}, 1), stopped: make(chan error, 1)}
}

func (f *fakeRecorder) Start(context.Context, map[string]string) {
	select {
	case f.started <- struct{}{}:
	default:
	}
}

func (f *fakeRecorder) Stop(_ context.Context, err error) {
	select {
	case f.stopped <- err:
	default:
	}
}

func (f *fakeRecorder) Connected(context.Context)           {}
func (f *fakeRecorder) Disconnected(context.Context, error) {}
func (f *fakeRecorder) Count(context.Context, string, int64, map[string]string) {
}
func (f *fakeRecorder) Observe(context.Context, string, float64, map[string]string) {}

// SetRecorder 装配自定义 recorder 后 Run 应触发 Start；ctx 取消后应触发 Stop
// 且 Run 返回（capture.enabled=false 时 Run=注册上线+等 ctx，退出路径最短）。
func TestSetRecorder_LifecycleStartStopObserved(t *testing.T) {
	cfg := config.Config{Agent: config.Agent{ID: "cap-tel", GameID: "g", Env: "test"}}
	r := New(cfg, nil)

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
	case err := <-done:
		require.NoError(t, err)
	case <-rec.stopped:
		// Stop 先于 Run 返回触发（defer 序），等 done 兜底即可。
		<-done
	case <-time.After(5 * time.Second):
		t.Fatal("expected Run to return after cancel")
	}
}

// SetRecorder(nil) 回退 noop（默认关闭=零开销路径契约）。
func TestSetRecorder_NilFallsBackToNoop(t *testing.T) {
	cfg := config.Config{Agent: config.Agent{ID: "cap-tel", GameID: "g", Env: "test"}}
	r := New(cfg, nil)
	r.SetRecorder(nil)
	require.NotNil(t, r.rec)
}
