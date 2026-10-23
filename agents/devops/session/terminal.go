package session

import (
	"errors"
	"io"
)

// Terminal 交互式终端会话（SSH 通道实现；Hub 数据面消费的最小契约）。
// 与 Session 的关系：Session 是建立链产物（ID/Close 生命周期），Terminal
// 在其上暴露双向数据流与窗口调整——RDP/VNC 等 guacd 通道另批实现时同样
// 落在本接口，Hub 不感知具体协议。
type Terminal interface {
	Session

	// Stdin 终端输入写入端（键盘数据；写入阻塞受 SSH 流控自然背压）。
	Stdin() io.Writer
	// Stdout 终端输出读出端（含回显；EOF = 远端会话结束）。
	Stdout() io.Reader
	// Stderr 终端错误输出读出端（EOF = 远端会话结束）。
	Stderr() io.Reader
	// Resize 调整远端 pty 窗口（cols/rows 合法域见 ValidateWindow）。
	Resize(cols, rows int) error
	// Wait 阻塞至远端会话结束（EOF/退出/通道错误），返回退出状态。
	// Close 之后 Wait 必须返回（本地断开也结束等待）。
	Wait() (TerminalExit, error)
}

// TerminalExit 远端会话退出状态（exit code 无法取得时 -1）。
type TerminalExit struct {
	ExitCode int
}

// ErrSessionBusy 输入队列满载（背压面：调用方退避重试，不丢弃键盘数据）。
var ErrSessionBusy = errors.New("session: input queue full, retry later")

// ValidateWindow pty 窗口合法域（1..1000，防 0/负数与病态巨型窗口）。
func ValidateWindow(cols, rows int) error {
	if cols < 1 || cols > 1000 || rows < 1 || rows > 1000 {
		return errors.New("session: window size out of range (1..1000)")
	}
	return nil
}

// DefaultWindow pty 初始窗口（请求未带 cols/rows 时的缺省）。
const (
	DefaultCols = 80
	DefaultRows = 24
)
