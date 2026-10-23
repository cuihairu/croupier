package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionMeta 会话存档条目（JSONL 追加，事件两态：started/ended）。会话
// 内容回放存档为 asciicast v2（casts/<id>.cast，asciinema 直接可回放）；
// 本文件是会话索引——「谁、何时、连哪台、多久、回放档在哪、多大」。
type SessionMeta struct {
	Event         string `json:"event"` // started | ended
	SessionID     string `json:"sessionId"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Operator      string `json:"operator"`
	Channel       string `json:"channel"` // provider 名（ssh / guacd / …）
	StartedUnixMs int64  `json:"startedUnixMs"`
	EndedUnixMs   int64  `json:"endedUnixMs,omitempty"`
	Outcome       string `json:"outcome,omitempty"` // ended 态：closed | eof | error | cast-broken
	// 回放存档收尾统计（ended 态；File 为相对声明——运维按 SessionDir 拼）。
	CastFile   string `json:"castFile,omitempty"`
	CastEvents int    `json:"castEvents,omitempty"`
	CastBytes  int64  `json:"castBytes,omitempty"`
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

// EndStats 记会话结束并携带回放存档收尾统计（Hub 断线清理用；stats.File
// 只存文件名——目录由 SessionDir 契约定位）。
func (r *Recorder) EndStats(sessionID, outcome string, stats CastStats) error {
	if r == nil || r.f == nil {
		return fmt.Errorf("session: recorder is closed")
	}
	file := ""
	if stats.File != "" {
		file = filepath.Base(stats.File)
	}
	return r.append(SessionMeta{
		Event:       "ended",
		SessionID:   sessionID,
		EndedUnixMs: time.Now().UnixMilli(),
		Outcome:     outcome,
		CastFile:    file,
		CastEvents:  stats.Events,
		CastBytes:   stats.Bytes,
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
