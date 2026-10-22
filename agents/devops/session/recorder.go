package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionMeta 会话存档条目（JSONL 追加，事件两态：started/ended）。会话内容
// 存档（SSH 命令流 / RDP·VNC 录像帧）的格式与保留期另批报批（agent-core
// §7.3）；骨架期先落起止元数据——「谁、何时、连哪台、多久」在协议通道到位
// 前即可审计。
type SessionMeta struct {
	Event         string `json:"event"` // started | ended
	SessionID     string `json:"sessionId"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Operator      string `json:"operator"`
	Channel       string `json:"channel"` // provider 名（guacd / local-ssh / noop）
	StartedUnixMs int64  `json:"startedUnixMs"`
	EndedUnixMs   int64  `json:"endedUnixMs,omitempty"`
	Outcome       string `json:"outcome,omitempty"` // ended 态：closed | close-error
}

// Recorder 会话存档写入器（JSONL 追加写；单文件 sessions.jsonl）。
type Recorder struct {
	mu sync.Mutex
	f  *os.File
}

// OpenRecorder 打开会话存档（目录不存在即建）。
func OpenRecorder(dir string) (*Recorder, error) {
	if dir == "" {
		return nil, fmt.Errorf("session: recorder dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("session: create recorder dir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "sessions.jsonl"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("session: open archive: %w", err)
	}
	return &Recorder{f: f}, nil
}

// append 落一行 JSONL（audit 真值同款语义：追加成功即留痕）。
func (r *Recorder) append(meta SessionMeta) error {
	if r == nil || r.f == nil {
		return fmt.Errorf("session: recorder is closed")
	}
	line, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("session: marshal meta: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("session: append archive: %w", err)
	}
	return nil
}

// Start 记会话开始（error 仅返回给调用方记日志，不阻断已建立的会话）。
func (r *Recorder) Start(meta SessionMeta) error {
	meta.Event = "started"
	return r.append(meta)
}

// End 记会话结束（幂等语义靠事件行本身，不做状态机）。
func (r *Recorder) End(sessionID string, outcome string) error {
	if r == nil || r.f == nil {
		return fmt.Errorf("session: recorder is closed")
	}
	return r.append(SessionMeta{
		Event:       "ended",
		SessionID:   sessionID,
		EndedUnixMs: time.Now().UnixMilli(),
		Outcome:     outcome,
	})
}

// Close 关闭存档文件（幂等）。
func (r *Recorder) Close() error {
	if r == nil || r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}
