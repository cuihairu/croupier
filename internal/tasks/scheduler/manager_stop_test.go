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

// ---- ARCH-017 / R30-07 补齐：不配合 lifecycle ctx 的卡死（硬界兜底）----

// tickBlock 阻塞在 ch 上的 tick 钩子：模拟「调度 tick 卡死在既不观察
// lifecycle ctx、也无截止的调用里」——lifecycle ctx 取消救不了它，Stop 的
// 硬上限必须兜底。
func tickBlock(ch <-chan struct{}) func() {
	return func() { <-ch }
}

func TestManagerStopBoundedWhenTickStuck(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	block := make(chan struct{})
	m.SetTickHook(tickBlock(block))
	m.SetStopTimeout(200 * time.Millisecond)
	m.SetInterval(time.Millisecond) // 首 tick 立即触发并卡死
	m.Start()

	done := make(chan struct{})
	go func() { m.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop hung on a stuck tick (unbounded wait)")
	}

	// 循环此刻仍卡在钩子上；释放后必须自行退出（无泄漏）。
	close(block)
	select {
	case <-m.stoppedCh:
	case <-time.After(3 * time.Second):
		t.Fatal("loop did not exit after unblocking the stuck tick")
	}
}

func TestManagerStopSecondCallerBoundedWhenTickStuck(t *testing.T) {
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	block := make(chan struct{})
	m.SetTickHook(tickBlock(block))
	m.SetStopTimeout(150 * time.Millisecond)
	m.SetInterval(time.Millisecond)
	m.Start()

	// 模拟 runServer：优雅停机同步 Stop + defer 兜底二次 Stop，二者都须有界。
	done := make(chan struct{})
	go func() {
		m.Stop() // 优雅停机同步调用（超时返回，停机继续推进）
		m.Stop() // defer 兜底（不得二次无限等待）
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("second Stop hung on a stuck tick")
	}
	close(block)
	select {
	case <-m.stoppedCh:
	case <-time.After(3 * time.Second):
		t.Fatal("loop did not exit after unblocking")
	}
}

func TestManagerStopUnblocksBeforeTimeoutWhenTickHealthy(t *testing.T) {
	// 正常路径：tick 不被阻塞，Stop 应在停止上限内、且远早于上限返回
	// （验证有界等待不把健康循环拖满 stopTimeout）。
	m := NewManager(newFakeStore(), &fakeDispatcher{})
	m.SetStopTimeout(5 * time.Second)
	m.SetInterval(time.Hour) // 无 tick 干扰：循环在 select 上等 stopCh
	m.Start()

	start := time.Now()
	m.Stop()
	require.WithinDuration(t, start, time.Now(), 2*time.Second,
		"Stop should return promptly when the loop is healthy")
}
