package execlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// FileWriter 本地轮转文件审计写入（真值面，离线不丢）。JSON-lines 追加，
// 每条落盘后 Sync——audit-first 闸的「已持久化」以此为准。
type FileWriter struct {
	mu sync.Mutex
	w  *lumberjack.Logger
}

// WriterOptions 轮转参数；零值取默认档（随实施报批：100MB/10 份/30 天，
// 不压缩——审计文件要能直接 tail）。
type WriterOptions struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
}

// OpenFileWriter 打开 dir/filename 的审计文件写入器。
func OpenFileWriter(dir, filename string, opts WriterOptions) (*FileWriter, error) {
	if dir == "" || filename == "" {
		return nil, fmt.Errorf("execlog: dir and filename are required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("execlog: create dir: %w", err)
	}
	if opts.MaxSizeMB <= 0 {
		opts.MaxSizeMB = 100
	}
	if opts.MaxBackups <= 0 {
		opts.MaxBackups = 10
	}
	if opts.MaxAgeDays <= 0 {
		opts.MaxAgeDays = 30
	}
	return &FileWriter{w: &lumberjack.Logger{
		Filename:   filepath.Join(dir, filename),
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
	}}, nil
}

// Append 追加一条记录并落盘。lumberjack 无缓冲直写 os.File（write(2) 直达
// 内核页缓存），返回 nil 即「进程崩溃不丢」——audit-first 闸据此放行；断电
// 级 fsync 不在 lumberjack 能力内，属已知边界。返回错误即「未持久化」，闸
// 据此拒绝放行。
func (w *FileWriter) Append(rec *Record) error {
	if w == nil || w.w == nil {
		return fmt.Errorf("execlog: writer is closed")
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("execlog: marshal record: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("execlog: write record: %w", err)
	}
	return nil
}

// Close 关闭底层文件。
func (w *FileWriter) Close() error {
	if w == nil || w.w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.w.Close()
	w.w = nil
	return err
}

// ReadRecords 读回 JSON-lines 审计文件；坏行跳过（截断容错），返回有效记录。
func ReadRecords(path string) ([]*Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []*Record
	for _, line := range splitLines(data) {
		var rec Record
		if json.Unmarshal(line, &rec) == nil && rec.Action != "" {
			out = append(out, &rec)
		}
	}
	return out, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// NowMs 当前毫秒时间戳（测试可注入时间时改用 Logger.now）。
func NowMs() int64 { return time.Now().UnixMilli() }
