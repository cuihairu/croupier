// ARCH-017：Manager.Stop 幂等性——优雅停机在关池前显式 StopScheduler，
// 同时保留 runServer 的 defer 兜底，二次调用必须安全（不得 close 已关闭的
// stopCh 或阻塞在 stoppedCh）。
package scheduler

import (
	"testing"
	"time"

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
