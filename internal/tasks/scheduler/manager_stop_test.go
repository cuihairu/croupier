// ARCH-017：Manager.Stop 幂等性——优雅停机在关池前显式 StopScheduler，
// 同时保留 runServer 的 defer 兜底，二次调用必须安全（不得 close 已关闭的
// stopCh 或阻塞在 stoppedCh）。
package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

func TestManagerStopIdempotent(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	m.Start()
	m.Stop()

	// 二次 Stop：不得 panic（close of closed channel），且必须在有界时间返回。
	done := make(chan struct{})
	go func() { m.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second Stop hung on stoppedCh")
	}

	// Start 后已 Stop 的 manager 不得被 Start 重启（保持停止态）。
	m.Start()
	done2 := make(chan struct{})
	go func() { m.Stop(); close(done2) }()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop after re-Start hung")
	}
}

func TestManagerStopBeforeStart(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	m.Stop() // 未 Start：no-op，不 panic 不阻塞
	m.Stop()
	require.False(t, m.started)
}

// TestManagerStop_AbortsBlockedTick verifies ARCH-017 R30-07: a tick blocked
// inside a store/dispatch call on the lifecycle context is aborted by Stop,
// so Stop returns within a bounded time instead of hanging past the 30s
// shutdown budget (the loop previously used context.Background(), which a
// hung store call never observes).
func TestManagerStop_AbortsBlockedTick(t *testing.T) {
	entered := make(chan struct{})
	blockStore := &blockingStore{entered: entered}
	m := NewManager(blockStore, &fakeDispatcher{})
	m.SetInterval(5 * time.Millisecond)
	m.Start()

	// Wait until the first tick has entered the blocking store call.
	<-entered

	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()
	select {
	case <-done:
		// Stop must return promptly: the lifecycle context is canceled, the
		// blocked ListDue observes cancellation and the loop exits.
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung on a blocked tick; lifecycle context was not honored")
	}
}

// blockingStore is a ScheduleStore whose ListDue blocks until the context is
// canceled (simulating a hung DB query during shutdown).
type blockingStore struct {
	entered chan struct{}
}

func (b *blockingStore) ListDue(ctx context.Context, _ time.Time, _ int) ([]model.TaskSchedule, error) {
	close(b.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}
func (b *blockingStore) UpdateSchedule(context.Context, uint, map[string]interface{}) error {
	return nil
}
func (b *blockingStore) HasRunLog(context.Context, uint, time.Time) (bool, error) { return false, nil }
func (b *blockingStore) CreateRunLog(context.Context, *model.TaskScheduleRunLog) (bool, error) {
	return false, nil
}
func (b *blockingStore) GetRunLog(context.Context, uint, time.Time) (*model.TaskScheduleRunLog, error) {
	return nil, nil
}
func (b *blockingStore) UpdateRunLog(context.Context, uint, time.Time, string, string, string) error {
	return nil
}
func (b *blockingStore) ListStaleClaims(context.Context, time.Time, int) ([]model.TaskScheduleRunLog, error) {
	return nil, nil
}
func (b *blockingStore) LastRunStatus(context.Context, string) (string, error) { return "", nil }
