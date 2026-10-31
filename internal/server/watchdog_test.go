package server

// ARCH-018 D3 看门狗收割器测试：runTaskWatchdog 触发、nil reaper no-op、
// taskWatchdogLoop ticker 调度分支、watchdogIntervalFor 钳位。

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/stretchr/testify/require"
)

type mockTaskReaper struct {
	calls   int32 // atomic
	rows    int64
	budget  time.Duration
	status  string
	message string
}

func (m *mockTaskReaper) ReapStale(ctx context.Context, budget time.Duration, status, message string) (int64, error) {
	atomic.AddInt32(&m.calls, 1)
	m.budget = budget
	m.status = status
	m.message = message
	return m.rows, nil
}

func TestControlService_RunTaskWatchdog_Fires(t *testing.T) {
	svc := newTestControlService()
	reaper := &mockTaskReaper{rows: 2}
	svc.SetTaskReaper(reaper, 4*time.Second)

	svc.runTaskWatchdog()

	require.EqualValues(t, 1, atomic.LoadInt32(&reaper.calls))
	require.Equal(t, 4*time.Second, reaper.budget)
	require.Equal(t, string(tasks.StatusTimedOut), reaper.status)
	require.Equal(t, "no event past watchdog budget; execution state unknown", reaper.message)
	// SetTaskReaper 同步推导收割间隔。
	require.Equal(t, 2*time.Second, svc.watchdogInterval)
}

func TestControlService_RunTaskWatchdog_NoReaper(t *testing.T) {
	svc := newTestControlService()
	// nil reaper：no-op，不 panic（后台循环只在 reaper 装配时启动）。
	require.NotPanics(t, func() { svc.runTaskWatchdog() })
}

func TestControlService_TaskWatchdogLoop_TickFires(t *testing.T) {
	// 钳位下限注入 1ms，使 budget 4ms → interval 2ms，快速 tick。
	origMin := watchdogIntervalMin
	watchdogIntervalMin = time.Millisecond
	t.Cleanup(func() { watchdogIntervalMin = origMin })

	svc := newTestControlService()
	reaper := &mockTaskReaper{}
	svc.SetTaskReaper(reaper, 4*time.Millisecond)
	require.Equal(t, 2*time.Millisecond, svc.watchdogInterval)

	done := make(chan struct{})
	go func() {
		svc.taskWatchdogLoop()
		close(done)
	}()

	waitLoopCond(t, func() bool { return atomic.LoadInt32(&reaper.calls) >= 1 })

	svc.cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("taskWatchdogLoop did not exit after cancel")
	}
	require.Equal(t, string(tasks.StatusTimedOut), reaper.status)
}

func TestWatchdogIntervalFor_Clamps(t *testing.T) {
	require.Equal(t, 2*time.Second, watchdogIntervalFor(4*time.Second))
	require.Equal(t, watchdogIntervalMin, watchdogIntervalFor(time.Millisecond))
	require.Equal(t, watchdogIntervalMax, watchdogIntervalFor(10*time.Minute))
	require.Equal(t, watchdogIntervalMax, watchdogIntervalFor(0))
	require.Equal(t, watchdogIntervalMax, watchdogIntervalFor(-time.Second))
}

// 发现 7：SetTaskReaper 在写锁下写 watchdogBudget/watchdogInterval，而
// runTaskWatchdog 与 taskWatchdogLoop 后台循环此前无锁读。虽因「须在
// StartBackgroundTasks 前调用」靠 goroutine happens-before 免于现行 race，
// 字段读写无同步契约。修复后读侧加 RLock，与写锁对齐（race 检测干净）。
func TestControlService_WatchdogFields_RaceFreeRead(t *testing.T) {
	svc := newTestControlService()
	reaper := &mockTaskReaper{rows: 1}
	svc.SetTaskReaper(reaper, 4*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// 循环读（等价于后台 taskWatchdogLoop/runTaskWatchdog 的字段读）。
		for i := 0; i < 1000; i++ {
			svc.mu.RLock()
			_ = svc.watchdogBudget
			_ = svc.watchdogInterval
			_ = svc.taskReaper
			svc.mu.RUnlock()
			svc.runTaskWatchdog()
		}
	}()
	// 并发写（等价于 SetTaskReaper 在运行期被调用）。
	for i := 0; i < 100; i++ {
		svc.SetTaskReaper(reaper, time.Duration(i%10)*time.Second+time.Second)
	}
	<-done
	require.EqualValues(t, 1000, atomic.LoadInt32(&reaper.calls))
}
