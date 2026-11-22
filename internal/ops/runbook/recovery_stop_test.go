package runbook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// AUD-024：恢复循环须按服务生命周期共享并关闭。此前每请求 New 出的临时编排器
// 各自启动一个永不退出的后台 goroutine（等待 wake 直至进程退出）；Stop 必须能
// 收敛该 goroutine，否则服务退出后仍遗留 worker。

// TestRecoveryLoop_StopReleasesWorker：AUD-024 主断言——enqueue 启动循环后
// Stop 等待 done 返回（后台 goroutine 收敛退出），不遗留永不退出的 worker。
// 用恒失败 store 驱动循环进入退避等待（Stop 可中断该等待）。
func TestRecoveryLoop_StopReleasesWorker(t *testing.T) {
	loop := NewRecoveryLoop(&countingStore{fail: true})
	loop.enqueue(&pendingTerminal{runID: 1, status: "failed"})
	require.Eventually(t, loop.started.Load, time.Second, time.Millisecond)

	done := make(chan struct{})
	go func() { loop.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 未释放恢复后台 goroutine（等待 done 超时）")
	}
}

// TestRecoveryLoop_StopWithoutStartNoBlock：从未 enqueue（循环未启动、无后台
// goroutine）时 Stop 必须立即返回，不得阻塞等待不存在的 done。
func TestRecoveryLoop_StopWithoutStartNoBlock(t *testing.T) {
	loop := NewRecoveryLoop(&countingStore{fail: true})
	done := make(chan struct{})
	go func() { loop.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("未启动循环的 Stop 不得阻塞")
	}
}

// TestRecoveryLoop_StopIdempotent：重复 Stop 幂等（不 panic、不重复关闭）。
func TestRecoveryLoop_StopIdempotent(t *testing.T) {
	loop := NewRecoveryLoop(&countingStore{fail: true})
	loop.enqueue(&pendingTerminal{runID: 1, status: "failed"})
	require.Eventually(t, loop.started.Load, time.Second, time.Millisecond)
	require.NotPanics(t, func() { loop.Stop(); loop.Stop() })
}

// TestOrchestrator_WithRecoverySharesLoop：多个编排器经 WithRecovery 复用同一
// 循环实例（指针同源）；Stop 共享循环后其后台 worker 收敛。这保证服务层每请求
// 一个临时编排器时，全部复用唯一的服务级循环，而非各自启动 worker。
func TestOrchestrator_WithRecoverySharesLoop(t *testing.T) {
	store := &countingStore{fail: true}
	shared := NewRecoveryLoop(store)
	o1 := New(store, nil, nil, nil).WithRecovery(shared)
	o2 := New(store, nil, nil, nil).WithRecovery(shared)
	require.Same(t, shared, o1.recovery)
	require.Same(t, shared, o2.recovery)

	// 经 o1 触发 enqueue（启动共享循环）；Stop 共享循环后 worker 释放。
	o1.recovery.enqueue(&pendingTerminal{runID: 1, status: "failed"})
	require.Eventually(t, shared.started.Load, time.Second, time.Millisecond)
	done := make(chan struct{})
	go func() { shared.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("共享恢复循环未在 Stop 时释放")
	}
}
