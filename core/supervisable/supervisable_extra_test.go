package supervisable

// supervisable.go 补测：Main 的 Option 装配与心跳打点支线、watchSignals
// 的二次信号强退与优雅超时强退（exit 以注入桩捕获，不真退进程）、
// beatLoop 的打点失败双臂（首拍与周期拍）与 interval<=0 默认档。
//
// 无诚实跳过：exit 本身就是可注入缝，全部强退臂可达。

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubExit 把包级 exit 换成记录桩（不退进程）；返回还原函数。
func stubExit(t *testing.T) chan int {
	t.Helper()
	codes := make(chan int, 4)
	orig := exit
	exit = func(code int) { codes <- code }
	t.Cleanup(func() { exit = orig })
	return codes
}

func TestMainAppliesOptionsAndHeartbeat(t *testing.T) {
	hb := filepath.Join(t.TempDir(), "hb")
	ran := make(chan struct{})
	release := make(chan struct{})
	go func() {
		code := Main("test-agent", func(ctx context.Context) error {
			close(ran)
			<-release // 停留两个心跳周期后再干净返回
			return nil
		}, WithGracefulTimeout(3*time.Second), WithHeartbeat(hb, 20*time.Millisecond))
		require.Equal(t, 0, code)
	}()

	<-ran
	// 心跳打点在独立协程，等待打点文件落盘（调度时序不假设）
	require.Eventually(t, func() bool {
		info, err := os.Stat(hb)
		return err == nil && time.Since(info.ModTime()) < 2*time.Second
	}, 2*time.Second, 10*time.Millisecond, "heartbeat file must be created while running")
	close(release)
	time.Sleep(30 * time.Millisecond) // beatLoop 随 ctx 退出后再清 TempDir
}

func TestWatchSignalsRepeatedSignalForcesExit(t *testing.T) {
	codes := stubExit(t)
	sigCh := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go watchSignals("test-agent", sigCh, cancel, done, time.Second)

	sigCh <- syscall.SIGTERM
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("ctx not cancelled after first signal")
	}
	sigCh <- syscall.SIGTERM // 二次信号 → 强退
	select {
	case code := <-codes:
		require.Equal(t, 1, code)
	case <-time.After(time.Second):
		t.Fatal("repeated signal must force exit(1)")
	}
}

func TestWatchSignalsGracefulTimeoutForcesExit(t *testing.T) {
	codes := stubExit(t)
	sigCh := make(chan os.Signal, 2)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{}) // run 永不返回 → 走优雅超时
	go watchSignals("test-agent", sigCh, cancel, done, 20*time.Millisecond)

	sigCh <- syscall.SIGTERM
	select {
	case code := <-codes:
		require.Equal(t, 1, code)
	case <-time.After(2 * time.Second):
		t.Fatal("graceful timeout must force exit(1)")
	}
}

func TestBeatLoopFailureArmsAndDefaultInterval(t *testing.T) {
	// 父路径被文件占位 → 首拍失败（Warn 不 panic）；interval<=0 走默认档
	// （只覆盖分支，不等 10s 周期）
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	badPath := filepath.Join(occupied, "hb")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		beatLoop(ctx, badPath, 0) // interval<=0 → 默认 10s 档
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("beatLoop must stop on ctx cancel")
	}

	// 短周期 → 首拍与周期拍双双失败（周期拍错误臂）
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		beatLoop(ctx2, badPath, 5*time.Millisecond)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel2()
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("beatLoop must stop on ctx cancel")
	}
}
