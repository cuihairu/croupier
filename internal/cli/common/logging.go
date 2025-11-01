package common

import (
    "context"
    "io"
    "log"
    "log/slog"
    "os"
    "strings"

    lumberjack "gopkg.in/natefinch/lumberjack.v2"
    "sync/atomic"
    "github.com/spf13/viper"
)

// SetupLoggerWithFile configures both std log and slog default logger.
// format: console|json; level: debug|info|warn|error.
// If filePath != "", logs write to a rotating file.
func SetupLoggerWithFile(level, format, filePath string, maxSizeMB, maxBackups, maxAgeDays int, compress bool) {
    // writer
    var w io.Writer = os.Stderr
    if strings.TrimSpace(filePath) != "" {
        w = &lumberjack.Logger{Filename: filePath, MaxSize: maxSizeMB, MaxBackups: maxBackups, MaxAge: maxAgeDays, Compress: compress}
    }
    // slog handler
    var h slog.Handler
    lvl := slog.LevelInfo
    switch strings.ToLower(level) {
    case "debug": lvl = slog.LevelDebug
    case "warn": lvl = slog.LevelWarn
    case "error": lvl = slog.LevelError
    }
    opts := &slog.HandlerOptions{Level: lvl}
    if strings.ToLower(format) == "json" {
        h = slog.NewJSONHandler(w, opts)
    } else {
        h = slog.NewTextHandler(w, opts)
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

func (c *countHandler) Enabled(ctx context.Context, lvl slog.Level) bool { return c.next.Enabled(ctx, lvl) }
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
func (c *countHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return &countHandler{next: c.next.WithAttrs(attrs)} }
func (c *countHandler) WithGroup(name string) slog.Handler    { return &countHandler{next: c.next.WithGroup(name)} }

// GetLogCounters returns current log counters by level.
func GetLogCounters() map[string]int64 {
    d := cntDebug.Load(); i := cntInfo.Load(); w := cntWarn.Load(); e := cntError.Load()
    return map[string]int64{"debug": d, "info": i, "warn": w, "error": e, "total": d + i + w + e}
}

// MergeLogSection flattens a nested "log" section into top-level log.* keys.
func MergeLogSection(v *viper.Viper) {
    if sub := v.Sub("log"); sub != nil {
        for _, k := range []string{"level","format","file","max_size","max_backups","max_age","compress"} {
            if sub.IsSet(k) { v.Set("log."+k, sub.Get(k)) }
        }
    }
}
