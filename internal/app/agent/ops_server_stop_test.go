// 回归测试：OpsServer.Stop 曾对 managedProcess 写锁（mu.Lock）后误用
// 读解锁（mu.RUnlock），processes 非空时 Go 运行时直接 fatal
// "sync: RUnlock of unlocked RWMutex" 崩溃整个 agent 进程——生产路径
// Start() 登记 AutoRestart 进程后 Close()→Stop() 即触发，该循环体此前
// 因此从未被测试走过。
package agent

import (
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpsServerStopUnlocksWriteLock_K(t *testing.T) {
	s := NewOpsServer(nil, "agent-k-stop", "1.0.0", nil)

	// 直填一个最小托管进程（nil cmd + 已关闭 stopCh）：stopProcess 走
	// "already closed" 与 nil-cmd 两个早退，重点验证锁配对与状态翻转。
	p := &managedProcess{
		name:   "sleeper",
		state:  opsv1.ProcessState_PROCESS_STATE_RUNNING,
		stopCh: make(chan struct{}),
	}
	s.processes[p.name] = p

	// 修复前此调用 fatal 整个测试进程（RUnlock of unlocked RWMutex）。
	s.Stop()

	assert.Equal(t, opsv1.ProcessState_PROCESS_STATE_STOPPED, p.state, "process state must flip to STOPPED")

	// 锁确实已释放：可再次加锁（若 Stop 泄漏写锁，此处死锁超时失败）。
	p.mu.Lock()
	p.mu.Unlock() //nolint:staticcheck // 空临界区即断言本体：Stop 若泄漏写锁此处死锁超时

	// 幂等：Stop 不清空 map，二次调用对已停止进程（stopCh 已关、cmd nil）
	// 重复走早退路径，不 fatal 不 panic。
	require.NotPanics(t, s.Stop)
}
