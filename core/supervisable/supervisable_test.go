package supervisable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainCleanStop(t *testing.T) {
	ran := make(chan struct{})
	code := Main("test-agent", func(ctx context.Context) error {
		close(ran)
		return nil
	})
	assert.Equal(t, 0, code)
	select {
	case <-ran:
	default:
		t.Fatal("run was not executed")
	}
}

func TestMainErrorExitCode(t *testing.T) {
	code := Main("test-agent", func(ctx context.Context) error {
		return errors.New("boom")
	})
	assert.Equal(t, 1, code)
}

func TestMainContextCancelledOnSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go watchSignals("test-agent", sigCh, cancel, done, time.Second)

	sigCh <- syscall.SIGTERM
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("ctx not cancelled after signal")
	}
	close(done) // 模拟 run 返回，避免 watchSignals 走超时强退
}

func TestMainHeartbeatLoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "hb")
	ctx, cancel := context.WithCancel(context.Background())
	go beatLoop(ctx, path, 10*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	cancel()

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), info.ModTime(), time.Second)
}

func TestOptionsDefaultsAndOverrides(t *testing.T) {
	cfg := config{gracefulTimeout: 10 * time.Second, heartbeatInterval: 10 * time.Second}
	WithGracefulTimeout(0)(&cfg)
	assert.Equal(t, 10*time.Second, cfg.gracefulTimeout) // 非法值不覆盖
	WithGracefulTimeout(2 * time.Second)(&cfg)
	assert.Equal(t, 2*time.Second, cfg.gracefulTimeout)

	WithHeartbeat("", time.Second)(&cfg)
	assert.Empty(t, cfg.heartbeatPath)                  // 空 path 不开启
	assert.Equal(t, time.Second, cfg.heartbeatInterval) // interval 独立于 path 生效
	WithHeartbeat("/tmp/hb", 0)(&cfg)
	assert.Equal(t, "/tmp/hb", cfg.heartbeatPath)
	assert.Equal(t, time.Second, cfg.heartbeatInterval) // interval<=0 保持现值
}
