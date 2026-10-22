// 回归测试（能力矩阵批次 F 内部瑕疵）：monitorProcess 的 AutoRestart 重试
// 延迟曾在 p.mu 写锁内 time.Sleep——RestartDelay 窗口内 ListProcesses/
// GetProcess 等状态读者全部被堵满延迟时长。修复后延迟在锁外等待且可被
// stopCh 中断：本用例在 waitDone 关闭（Wait 收尸完成）后立即采样 p.mu，
// 修复前 monitor 恰在此后持锁睡满整个 RestartDelay，TryLock 必失败。
package agent

import (
	"context"
	"os"
	"testing"
	"time"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestOpsServerMonitorProcess_RestartDelayOutsideLock(t *testing.T) {
	if os.Getenv("CROUPIER_V9_SKIP_SLOW") != "" {
		t.Skip("slow test skipped by env")
	}
	s := NewOpsServer(&OpsConfig{
		Enabled:      true,
		AllowRestart: true,
		ManagedProcesses: map[string]ManagedProcessConfig{
			"boom": {Command: "false", AutoRestart: true, RestartDelay: 3 * time.Second},
		},
	}, "a", "v", nil)

	_, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "boom"})
	require.NoError(t, err)

	s.mu.RLock()
	p := s.processes["boom"]
	s.mu.RUnlock()
	require.NotNil(t, p)

	// `false` 立即退出；等 Wait 收尸完成后 monitor 才进入延迟等待——采样
	// 点对齐修复前的持锁起点（waitDone 关闭 → 原代码立刻 p.mu.Lock+Sleep）。
	select {
	case <-p.waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit in time")
	}
	require.True(t, p.mu.TryLock(), "p.mu must be free during the restart delay (lock-sleep regression)")
	p.mu.Unlock()

	// 状态读者在延迟窗口内可正常返回（修复前被堵满 3s RestartDelay）。
	procs, err := s.ListProcesses(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, procs.Processes, 1)

	// stopCh 中断剩余延迟：stopProcess 本身不翻状态（StopProcess handler/
	// Stop 循环才置 STOPPED），此处同口径收口，再越过原延迟时点验证中断
	// 生效——若 monitor 未被 stopCh 唤醒，3s 到点后会重启（restarts→2、
	// startProcess 重开 waitDone），该窗口过去后 restarts 必须仍为初值 1。
	s.stopProcess(p)
	p.mu.Lock()
	p.state = opsv1.ProcessState_PROCESS_STATE_STOPPED
	p.mu.Unlock()
	time.Sleep(3500 * time.Millisecond)
	p.mu.RLock()
	restarts := p.restarts
	p.mu.RUnlock()
	assert.Equal(t, int32(1), restarts, "stopped process must not be restarted after the delay elapses")
}
