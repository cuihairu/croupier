// Package logx 结构化日志装配（slog + lumberjack，矩阵 #11 拍板「留」）。
// 上收自 internal/cli/common/logging.go：console|json 双格式、可选轮转文件、
// 级别计数；std log 桥接同一 writer。server/agent 与 core 系 agent 共用。
package logx

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig 日志配置（通用结构，供所有服务使用）。
type LogConfig struct {
	Level      string `json:",omitempty" yaml:",omitempty"` // debug|info|warn|error
	Format     string `json:",omitempty" yaml:",omitempty"` // console|json
	Output     string `json:",omitempty" yaml:",omitempty"` // stdout|stderr
	File       string `json:",omitempty" yaml:",omitempty"` // 日志文件路径
	MaxSize    int    `json:",omitempty" yaml:",omitempty"` // 单个日志文件最大大小（MB）
	MaxBackups int    `json:",omitempty" yaml:",omitempty"` // 保留的旧日志文件最大数量
	MaxAge     int    `json:",omitempty" yaml:",omitempty"` // 保留旧日志文件的最大天数
	Compress   bool   `json:",omitempty" yaml:",omitempty"` // 是否压缩旧日志文件
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
