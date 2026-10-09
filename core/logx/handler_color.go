package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
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

	// 先构建完整的日志消息
	var buf []byte
	if r.Level >= slog.LevelError {
		buf = append(buf, colorRed...)
	} else if r.Level >= slog.LevelWarn {
		buf = append(buf, colorYellow...)
	} else if r.Level >= slog.LevelInfo {
		buf = append(buf, colorGreen...)
	} else {
		buf = append(buf, colorGray...)
	}

	// 添加级别
	buf = append(buf, r.Level.String()...)
	buf = append(buf, colorReset...)

	// 添加时间
	if !r.Time.IsZero() {
		buf = append(buf, ' ')
		buf = r.Time.AppendFormat(buf, "15:04:05.000")
	}

	// 添加消息
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)

	// 添加属性
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
		return true
	})

	buf = append(buf, '\n')

	// 写入输出
	_, err := h.w.Write(buf)
	return err
}

// isTerminal 检查文件是否是终端
func isTerminal(f *os.File) bool {
	fileInfo, _ := f.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
