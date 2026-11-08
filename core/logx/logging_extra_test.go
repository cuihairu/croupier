package logx

// logging.go / handler_count.go / handler_color.go 补测：stderr 路由双环境
// 变量（管道替身捕获）、级别闭集 warn/error 的过滤行为、日志目录创建失败
// 兜底、std log 桥接（writerFunc 闭包落文件）、彩色处理器属性渲染，以及
// countHandler 四级计数器增量。
//
// 无诚实跳过：全局 logger 状态由各用例自行 Setup 覆盖，互不依赖。

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// captureStderr 把 os.Stderr 换成管道读端并返回读取闭包（stderr 路由
// 断言用；用毕恢复）。
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stderr
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = old
		require.NoError(t, w.Close())
		require.NoError(t, r.Close())
	})
	return func() string {
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		return string(buf[:n])
	}
}

func TestColoredHandlerRendersAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := newColoredTextHandler(&buf, nil)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "with attrs", 0)
	record.AddAttrs(slog.String("game", "demo"), slog.Int("port", 18780))
	require.NoError(t, handler.Handle(context.Background(), record))

	out := buf.String()
	require.Contains(t, out, "game=demo")
	require.Contains(t, out, "port=18780")
	require.Contains(t, out, "\033[", "buffer writer is not a terminal, colored path expected")
}

func TestCountHandlerLevelCounters(t *testing.T) {
	var buf bytes.Buffer
	h := &countHandler{next: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
	before := GetLogCounters()

	for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		require.NoError(t, h.Handle(context.Background(),
			slog.NewRecord(time.Now(), lvl, "counted", 0)))
	}

	after := GetLogCounters()
	require.Equal(t, before["debug"]+1, after["debug"])
	require.Equal(t, before["info"]+1, after["info"])
	require.Equal(t, before["warn"]+1, after["warn"])
	require.Equal(t, before["error"]+1, after["error"])
	require.Equal(t, before["total"]+4, after["total"])
}

func TestSetupLoggerStdLogBridgeToFile(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "bridge.log")
	SetupLoggerWithFile("info", "console", logFile, 10, 3, 7, false)

	slog.Info("slog line")
	log.Printf("std line")

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "slog line")
	require.Contains(t, string(data), "std line", "std log must bridge to the same writer")
}

func TestSetupLoggerLevelFiltering(t *testing.T) {
	// warn 档：info 被滤掉，warn 保留
	warnFile := filepath.Join(t.TempDir(), "warn.log")
	SetupLoggerWithFile("warn", "console", warnFile, 10, 3, 7, false)
	slog.Info("filtered info")
	slog.Warn("kept warn")
	data, err := os.ReadFile(warnFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "kept warn")
	require.NotContains(t, string(data), "filtered info")

	// error 档：warn 被滤掉，error 保留
	errFile := filepath.Join(t.TempDir(), "error.log")
	SetupLoggerWithFile("error", "console", errFile, 10, 3, 7, false)
	slog.Warn("filtered warn")
	slog.Error("kept error")
	data, err = os.ReadFile(errFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "kept error")
	require.NotContains(t, string(data), "filtered warn")
}

func TestSetupLoggerRoutesStderr(t *testing.T) {
	// LOG_OUTPUT=stderr
	t.Setenv("LOG_OUTPUT", "stderr")
	read := captureStderr(t)
	SetupLoggerWithFile("info", "console", "", 10, 3, 7, false)
	slog.Info("to stderr via LOG_OUTPUT")
	require.Contains(t, read(), "to stderr via LOG_OUTPUT")

	// CROUPIER_LOG_OUTPUT=stderr（LOG_OUTPUT 已_unset，走第二优先级）
	os.Unsetenv("LOG_OUTPUT")
	t.Setenv("CROUPIER_LOG_OUTPUT", "stderr")
	SetupLoggerWithFile("info", "console", "", 10, 3, 7, false)
	slog.Info("to stderr via CROUPIER_LOG_OUTPUT")
	require.Contains(t, read(), "to stderr via CROUPIER_LOG_OUTPUT")
}

func TestSetupLoggerMkdirAllFallback(t *testing.T) {
	// 父路径被文件占位 → 目录创建失败 → 仅 console 输出的兜底提示
	occupied := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(occupied, []byte("x"), 0o600))
	SetupLoggerWithFile("info", "console", filepath.Join(occupied, "sub", "app.log"),
		10, 3, 7, false)
	slog.Info("console only after mkdir failure") // 不 panic 即可
}
