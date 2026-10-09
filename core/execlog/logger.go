package execlog

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Uploader 把审计记录上报 server 落审计存储（按 scope 归 game 库，沿用审计链
// 完整性机制）。实现方负责重试与断点续传；本地文件才是真值。
type Uploader interface {
	UploadExecLog(ctx context.Context, rec *Record) error
}

// UploaderFunc 适配函数形态的 Uploader。
type UploaderFunc func(ctx context.Context, rec *Record) error

// UploadExecLog implements Uploader.
func (f UploaderFunc) UploadExecLog(ctx context.Context, rec *Record) error {
	return f(ctx, rec)
}

// Logger 双写门面：本地文件为真值（失败向上传播），上报尽力而为（失败只
// 计数与回调，绝不阻断执行）。观测类轮询直接用 Record；命令类执行经 Gate。
type Logger struct {
	w              *FileWriter
	uploader       Uploader
	uploadTimeout  time.Duration
	now            func() time.Time
	uploadFailures atomic.Int64
	onUploadError  func(error)
}

// Option 装配 Logger 的可选项。
type Option func(*Logger)

// WithUploader 设置 server 上报通道（server 侧摄取端点为剩余项）。
func WithUploader(up Uploader) Option {
	return func(l *Logger) { l.uploader = up }
}

// WithUploadTimeout 单次上报超时，默认 5s。
func WithUploadTimeout(d time.Duration) Option {
	return func(l *Logger) { l.uploadTimeout = d }
}

// WithUploadErrorHook 上报失败回调（告警接入点，herald 消费）。
func WithUploadErrorHook(fn func(error)) Option {
	return func(l *Logger) { l.onUploadError = fn }
}

// New 装配双写 Logger。w 必填。
func New(w *FileWriter, opts ...Option) *Logger {
	l := &Logger{w: w, uploadTimeout: 5 * time.Second, now: func() time.Time { return time.Now() }}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Record 落一条执行记录（观测类轮询、命令执行结果均走此口）。本地持久化
// 失败时错误向上传播（真值不丢），上报失败只计数。
func (l *Logger) Record(rec *Record) error {
	if l == nil || l.w == nil {
		return fmt.Errorf("execlog: logger is closed")
	}
	if rec.TsUnixMs == 0 {
		rec.TsUnixMs = l.now().UnixMilli()
	}
	if !rec.ScopeRequired() {
		return fmt.Errorf("execlog: gameId and env are required (scope contract)")
	}
	if err := l.w.Append(rec); err != nil {
		return err
	}
	l.tryUpload(rec)
	return nil
}

func (l *Logger) tryUpload(rec *Record) {
	if l.uploader == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), l.uploadTimeout)
	defer cancel()
	if err := l.uploader.UploadExecLog(ctx, rec); err != nil {
		l.uploadFailures.Add(1)
		if l.onUploadError != nil {
			l.onUploadError(err)
		}
	}
}

// UploadFailures 累计上报失败次数（面板/告警读数）。
func (l *Logger) UploadFailures() int64 {
	if l == nil {
		return 0
	}
	return l.uploadFailures.Load()
}

// Gate 返回 audit-first 放行闸（命令类执行专用）。
func (l *Logger) Gate() *Gate { return &Gate{l: l} }
