// 本轮覆盖率冲刺补充用例：
//   - sysinfo.go：systemdDetectPath 注入覆盖非 systemd 分支（"unknown"）
//   - ops_server.go：stopProcessWaitTimeout 注入 + 无人 close 的 waitDone
//     覆盖等待超时分支
//   - upstream.go：stopAndResetTimer 排水模式的已触发/未触发两分支
package agent

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLinuxServiceManager_PathInjected(t *testing.T) {
	orig := systemdDetectPath
	defer func() { systemdDetectPath = orig }()

	// 指向不存在的路径 → 走 "unknown" 分支。
	systemdDetectPath = t.TempDir() + "/no-such-systemd"
	assert.Equal(t, "unknown", detectLinuxServiceManager())

	// 指向真实存在的目录 → 恢复 "systemd" 判定，证明探测逻辑本身未变形。
	systemdDetectPath = t.TempDir()
	assert.Equal(t, "systemd", detectLinuxServiceManager())
}

func TestStopProcess_WaitDoneTimeoutBranch(t *testing.T) {
	orig := stopProcessWaitTimeout
	defer func() { stopProcessWaitTimeout = orig }()
	stopProcessWaitTimeout = 20 * time.Millisecond

	// 构造一个真实存活但无人收尸（无 monitorProcess、waitDone 永不关闭）
	// 的托管进程：stopProcess 在 Kill 后只能走等待超时分支。
	cmd := exec.Command("sleep", "5")
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Wait() }()

	p := &managedProcess{
		name:     "orphan-sleeper",
		cmd:      cmd,
		waitDone: make(chan struct{}), // 无人 close
	}
	s := NewOpsServer(nil, "agent-stop-timeout", "1.0.0", nil)

	start := time.Now()
	s.stopProcess(p)
	elapsed := time.Since(start)

	// 超时分支生效：既不能立刻返回（waitDone 未关闭），也不应等满默认 5s。
	assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond, "must have waited the injected timeout")
	assert.Less(t, elapsed, 3*time.Second, "must not wait the default 5s timeout")
}

func TestStopAndResetTimer_FiredTimer(t *testing.T) {
	// timer 已触发但值未被消费：Stop() 返回 false，排水分支清掉旧值，
	// Reset 后的短窗口内 timer.C 不应有值。
	timer := time.NewTimer(time.Millisecond)
	time.Sleep(20 * time.Millisecond) // 确保 fire

	stopAndResetTimer(timer, 10*time.Second)

	select {
	case <-timer.C:
		t.Fatal("stale fired value must be drained before Reset")
	default:
	}
}

func TestStopAndResetTimer_ActiveTimer(t *testing.T) {
	// timer 未触发：Stop() 返回 true（不走排水），Reset 后新周期到期可读到值。
	timer := time.NewTimer(time.Minute)

	stopAndResetTimer(timer, 10*time.Millisecond)

	select {
	case <-timer.C:
	case <-time.After(2 * time.Second):
		t.Fatal("reset timer must fire after the new duration")
	}
}
