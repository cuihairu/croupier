package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// asciicast v2（asciinema 回放格式）：每行一个 JSON 对象——首行 header、
// 之后每行一个事件 {time, eventtype, eventdata}。选它做会话内容回放存档
// 的原因：开放格式、asciinema play/cat 直接消费、时间轴单调可裁剪。
// 本文件只产不读（回放消费方是 asciinema/前端播放器）。
//
// 边界（诚实）：存档内容为 SSH 终端输出事件流（含服务端回显的输入），非
// 键盘事件级录制；asciicast 的 'i'（input）事件不落——终端回显已覆盖审计
// 可读性，且键盘级记录会引入密码等敏感输入落盘面。

// CastHeader asciicast v2 header（asciinema-2.0 规范字段）。
type CastHeader struct {
	Version   int               `json:"version"` // 恒 2
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Title     string            `json:"title,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// castEvent asciicast v2 事件行。Data 分两形：
//   - "o"（output）：JSON 字符串（原样终端字节）
//   - "r"（resize）：JSON 数组 [cols, rows]
type castEvent struct {
	Time  float64         `json:"time"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"eventdata"`
}

// CastWriter 单会话 asciicast 写入器（ casts/<sessionID>.cast，JSONL 追加）。
type CastWriter struct {
	mu    sync.Mutex
	f     *os.File
	bw    *bufio.Writer
	start time.Time
	mono  func() time.Time // 测试注入
	count int
	bytes int64
	err   error // sticky：首次写失败后全部 no-op，会话面按「存档断」处理
}

// castFileName 会话 id → 存档文件名。
func castFileName(sessionID string) string { return sessionID + ".cast" }

// OpenCast 打开会话内容回放存档并落 header（dir 不存在即建）。
func OpenCast(dir, sessionID string, header CastHeader, now time.Time) (*CastWriter, error) {
	if dir == "" {
		return nil, fmt.Errorf("session: cast dir is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session: cast session id is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("session: create cast dir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, castFileName(sessionID)),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return nil, fmt.Errorf("session: open cast: %w", err)
	}
	if header.Version == 0 {
		header.Version = 2
	}
	w := &CastWriter{f: f, bw: bufio.NewWriter(f), start: now, mono: time.Now}
	line, merr := json.Marshal(header)
	if merr != nil {
		_ = f.Close()
		return nil, fmt.Errorf("session: marshal cast header: %w", merr)
	}
	if _, werr := w.bw.Write(append(line, '\n')); werr != nil {
		_ = f.Close()
		return nil, fmt.Errorf("session: write cast header: %w", werr)
	}
	return w, nil
}

// Output 追加一条终端输出事件（尽力而为；失败置 sticky，由 Hub 收束会话）。
func (w *CastWriter) Output(data []byte) {
	w.append("o", mustJSONString(string(data)))
}

// Resize 追加一条窗口调整事件（回放时终端尺寸跟随变化）。
func (w *CastWriter) Resize(cols, rows int) {
	raw, _ := json.Marshal([]int{cols, rows})
	w.append("r", raw)
}

func (w *CastWriter) append(eventType string, data json.RawMessage) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return
	}
	ev := castEvent{Time: w.mono().Sub(w.start).Seconds(), Event: eventType, Data: data}
	line, err := json.Marshal(ev)
	if err != nil {
		w.err = fmt.Errorf("session: marshal cast event: %w", err)
		return
	}
	if _, err := w.bw.Write(append(line, '\n')); err != nil {
		w.err = fmt.Errorf("session: write cast event: %w", err)
		return
	}
	w.count++
	w.bytes += int64(len(line) + 1)
}

// Events 已落盘事件数（含 header 前的内容事件）。
func (w *CastWriter) Events() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Err sticky 写错误（nil = 至今全部落盘成功）。
func (w *CastWriter) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// Close 冲刷并关闭存档（幂等；返回 flush 错误——sticky 错误优先）。
func (w *CastWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return w.err
	}
	ferr := w.bw.Flush()
	if ferr == nil {
		ferr = w.f.Close()
	} else {
		_ = w.f.Close()
	}
	w.f = nil
	if w.err != nil {
		return w.err
	}
	return ferr
}

// mustJSONString 文本 → JSON 字符串字面量（json.Marshal 对 string 恒成功）。
func mustJSONString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// CastProbe 回放存档的形状断言输入（测试与运维抽查共用）。
type CastProbe struct{}

// ValidateCastFile 校验 .cast 文件是合法 asciicast v2（header 版本=2、事件
// 行 time 非负、o 事件 data 为字符串、r 事件 data 为 [cols,rows]）。返回
// 事件数。回放样本实证的最小程序化入口——不依赖 asciinema 安装。
func ValidateCastFile(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) < 2 {
		return 0, fmt.Errorf("cast: expected header + at least one event, got %d lines", len(lines))
	}
	var header CastHeader
	if err := json.Unmarshal(lines[0], &header); err != nil {
		return 0, fmt.Errorf("cast: header: %w", err)
	}
	if header.Version != 2 {
		return 0, fmt.Errorf("cast: header version = %d, want 2", header.Version)
	}
	events := 0
	for i, line := range lines[1:] {
		var ev struct {
			Time  float64         `json:"time"`
			Event string          `json:"event"`
			Data  json.RawMessage `json:"eventdata"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			return events, fmt.Errorf("cast: event %d: %w", i, err)
		}
		if ev.Time < 0 {
			return events, fmt.Errorf("cast: event %d: negative time", i)
		}
		switch ev.Event {
		case "o":
			var s string
			if err := json.Unmarshal(ev.Data, &s); err != nil {
				return events, fmt.Errorf("cast: event %d: output data not a string", i)
			}
		case "r":
			var wh []int
			if err := json.Unmarshal(ev.Data, &wh); err != nil || len(wh) != 2 {
				return events, fmt.Errorf("cast: event %d: resize data not [cols,rows]", i)
			}
		default:
			return events, fmt.Errorf("cast: event %d: unknown event type %q", i, ev.Event)
		}
		events++
	}
	return events, nil
}
