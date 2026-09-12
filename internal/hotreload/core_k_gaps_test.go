package hotreload

// core_k_gaps_test.go 补齐 watchLoop 中 watcher.Errors 事件分支：
// fsnotify 的 Errors 是导出 channel，测试直接注入错误事件，
// 驱动 watchLoop 走到“File watcher error”日志路径。

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchLoopLogsWatcherError_K(t *testing.T) {
	sb := &syncedBuffer{}
	hrIface, err := NewHotReloader(&Config{WatchDirs: []string{}}, slog.New(slog.NewTextHandler(sb, nil)))
	require.NoError(t, err)
	hr := hrIface.(*croupierHotReloader)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		hr.watchLoop(ctx)
		close(done)
	}()

	// Errors channel 无缓冲；watchLoop 阻塞在 select 上会立即接收。
	go func() {
		hr.watcher.Errors <- errors.New("injected watcher error k")
	}()

	assert.Eventually(t, func() bool {
		return strings.Contains(sb.String(), "File watcher error")
	}, 3*time.Second, 10*time.Millisecond, "watcher error must be logged by watchLoop")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after context cancellation")
	}
	require.NoError(t, hr.Stop())
}
