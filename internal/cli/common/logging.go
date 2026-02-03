package common

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	"path/filepath"
)

// ANSI 颜色代码
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorGray   = "\033[90m"
)

// coloredTextHandler 是一个带颜色的文本处理器
type coloredTextHandler struct {
	slog.Handler
	w io.Writer
}

// newColoredTextHandler 创建一个新的彩色文本处理器
func newColoredTextHandler(w io.Writer, opts *slog.HandlerOptions) *coloredTextHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &coloredTextHandler{
		Handler: slog.NewTextHandler(w, opts),
		w:       w,
	}
}

// Handle 处理日志记录并添加颜色
func (h *coloredTextHandler) Handle(ctx context.Context, r slog.Record) error {
	// 检查是否是终端输出
	if f, ok := h.w.(*os.File); ok && !isTerminal(f) {
		return h.Handler.Handle(ctx, r)
	}

	// 创建带颜色的 Writer
	var color string
	switch r.Level {
	case slog.LevelDebug:
		color = colorGray
	case slog.LevelInfo:
		color = colorGreen // INFO 使用绿色而不是红色
	case slog.LevelWarn:
		color = colorYellow
	case slog.LevelError:
		color = colorRed
	default:
		color = ""
	}

	colorWriter := &colorWriter{
		w:     h.w,
		color: color,
	}

	// 使用带颜色的 writer 创建临时 TextHandler
	tmpHandler := slog.NewTextHandler(colorWriter, nil)
	return tmpHandler.Handle(ctx, r)
}

// colorWriter 包装 writer 并添加颜色
type colorWriter struct {
	w     io.Writer
	color string
}

func (cw *colorWriter) Write(p []byte) (n int, err error) {
	if cw.color == "" {
		return cw.w.Write(p)
	}
	// 在 level=INFO 这一行添加颜色
	// 简单实现：在输出前添加颜色代码，在行尾重置
	cw.w.Write([]byte(cw.color))
	defer cw.w.Write([]byte(colorReset))
	return cw.w.Write(p)
}

// isTerminal 检查文件是否是终端
func isTerminal(f *os.File) bool {
	fileInfo, _ := f.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// SetupLoggerWithFile configures both std log and slog default logger.
// format: console|json; level: debug|info|warn|error.
// If filePath != "", logs write to a rotating file.
func SetupLoggerWithFile(level, format, filePath string, maxSizeMB, maxBackups, maxAgeDays int, compress bool) {
	// choose console writer: default stdout (避免终端红色 stderr)
	var console io.Writer = os.Stdout
	if dest := strings.ToLower(os.Getenv("LOG_OUTPUT")); dest == "stderr" {
		console = os.Stderr
	}
	if v := os.Getenv("CROUPIER_LOG_OUTPUT"); strings.ToLower(v) == "stderr" {
		console = os.Stderr
	}
	// optional file writer
	var file io.Writer
	if strings.TrimSpace(filePath) != "" {
		// ensure parent dir exists to avoid silent failures in lumberjack writer
		if dir := filepath.Dir(filePath); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				// fallback: 仅输出到 console，并提示
				log.Printf("warn: create log dir failed: %v (using console output)", err)
			}
		}
		file = &lumberjack.Logger{Filename: filePath, MaxSize: maxSizeMB, MaxBackups: maxBackups, MaxAge: maxAgeDays, Compress: compress}
	}
	// dual write: console + file (若未配置 file 则仅 console)
	var w io.Writer
	if file != nil {
		w = io.MultiWriter(console, file)
	} else {
		w = console
	}
	// slog handler
	var h slog.Handler
	lvl := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if strings.ToLower(format) == "json" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		// 使用自定义的彩色文本处理器
		h = newColoredTextHandler(w, opts)
	}
	// wrap with counting handler
	h = &countHandler{next: h}
	slog.SetDefault(slog.New(h))
	// std log bridge to same writer (simple; keep std flags minimal when json)
	if strings.ToLower(format) == "json" {
		log.SetFlags(0)
	} else {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}
	log.SetOutput(writerFunc(func(p []byte) (int, error) { return w.Write(p) }))
}

type writerFunc func(p []byte) (n int, err error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// --------- counters for log levels ----------

var cntDebug, cntInfo, cntWarn, cntError atomic.Int64

type countHandler struct{ next slog.Handler }

func (c *countHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return c.next.Enabled(ctx, lvl)
}
func (c *countHandler) Handle(ctx context.Context, rec slog.Record) error {
	switch rec.Level {
	case slog.LevelDebug:
		cntDebug.Add(1)
	case slog.LevelInfo:
		cntInfo.Add(1)
	case slog.LevelWarn:
		cntWarn.Add(1)
	case slog.LevelError:
		cntError.Add(1)
	}
	return c.next.Handle(ctx, rec)
}
func (c *countHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &countHandler{next: c.next.WithAttrs(attrs)}
}
func (c *countHandler) WithGroup(name string) slog.Handler {
	return &countHandler{next: c.next.WithGroup(name)}
}

// GetLogCounters returns current log counters by level.
func GetLogCounters() map[string]int64 {
	d := cntDebug.Load()
	i := cntInfo.Load()
	w := cntWarn.Load()
	e := cntError.Load()
	return map[string]int64{"debug": d, "info": i, "warn": w, "error": e, "total": d + i + w + e}
}

// MergeLogSection flattens a nested "log" section into top-level log.* keys.
func MergeLogSection(v *viper.Viper) {
	if sub := v.Sub("log"); sub != nil {
		for _, k := range []string{"level", "format", "file", "max_size", "max_backups", "max_age", "compress", "output"} {
			if sub.IsSet(k) {
				v.Set("log."+k, sub.Get(k))
			}
		}
	}
}
