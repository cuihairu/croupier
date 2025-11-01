package common

import (
    "io"
    "log"
    "log/slog"
    "os"
    "strings"

    lumberjack "gopkg.in/natefinch/lumberjack.v2"
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
