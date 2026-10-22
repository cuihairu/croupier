package logx

import (
	"context"
	"log/slog"
	"sync/atomic"
)

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
