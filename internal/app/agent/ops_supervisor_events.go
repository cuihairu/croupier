package agent

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"google.golang.org/protobuf/encoding/protojson"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// 事件类型闭集（SupervisorEvent.event；面板与文档共用此命名）。
const (
	supervisorEventDetectDown        = "detect_down"
	supervisorEventAutoRestart       = "auto_restart"
	supervisorEventRestartFailed     = "restart_failed"
	supervisorEventBreakerTripped    = "breaker_tripped"
	supervisorEventResourceOverLimit = "resource_over_limit"
	supervisorEventManualStart       = "manual_start"
	supervisorEventManualStop        = "manual_stop"
	// schedule_fire 是定时动作事件（§10.6 调度状态）：与 manual_* 分型，
	// 四类状态在事件流里可分辨（message 带 action 与 cron）。
	supervisorEventScheduleAction = "schedule_fire"
	// update_applied / update_rolled_back 是更新状态事件（§10.4）：message 带
	// 旧→新指纹；回滚同样留痕（rolledBackTo=旧指纹）。持久查询主入口=execlog。
	supervisorEventUpdateApplied    = "update_applied"
	supervisorEventUpdateRolledBack = "update_rolled_back"
	// schedule_paused / schedule_resumed 是调度暂停/恢复事件（§10.2 暂停与恢复
	// 暂停对象=调度）：维护窗口意图留痕，恢复同样留痕（message 带原因）。
	supervisorEventSchedulePaused  = "schedule_paused"
	supervisorEventScheduleResumed = "schedule_resumed"

	// flagBreakerTripped 追加在快照 flags 上（BROKEN 态恒带）。
	flagBreakerTripped = "breaker_tripped"
)

// supervisorEventRingSize 是 agent 侧事件环容量。server 侧内存环同为 500
// 上限（设计 §3.6）；agent 离线重连后一次上报最多补送这么多条。
const supervisorEventRingSize = 500

// supervisorEventLog is the agent-side supervisor event record: an in-memory
// ring for incremental report piggyback plus a rotating file mirror that is
// the full-truth log (survives agent restarts within retention).
type supervisorEventLog struct {
	mu     sync.Mutex
	seq    int64
	ring   []*opsv1.SupervisorEvent
	writer *lumberjack.Logger
	marsh  protojson.MarshalOptions
}

// newSupervisorEventLog builds the event log. The rotating file defaults to
// logs/supervisor/supervisor.log, 10MB × 5 backups × 7 days when the config
// leaves keys at zero.
func newSupervisorEventLog(cfg SupervisorLogConfig) *supervisorEventLog {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		dir = filepath.Join("logs", "supervisor")
	}
	maxSize := cfg.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 10
	}
	backups := cfg.MaxBackups
	if backups <= 0 {
		backups = 5
	}
	age := cfg.MaxAgeDays
	if age <= 0 {
		age = 7
	}
	return &supervisorEventLog{
		ring: make([]*opsv1.SupervisorEvent, 0, supervisorEventRingSize),
		writer: &lumberjack.Logger{
			Filename:   filepath.Join(dir, "supervisor.log"),
			MaxSize:    maxSize,
			MaxBackups: backups,
			MaxAge:     age,
		},
		marsh: protojson.MarshalOptions{},
	}
}

// emit assigns seq/ts, appends to the ring and mirrors one JSON line to the
// rotating file. File write errors are swallowed: the ring (and the panel)
// stays authoritative for the live view, the file is best-effort on disk.
func (l *supervisorEventLog) emit(ev *opsv1.SupervisorEvent) *opsv1.SupervisorEvent {
	l.mu.Lock()
	l.seq++
	ev.Seq = l.seq
	if ev.TsUnix == 0 {
		ev.TsUnix = time.Now().Unix()
	}
	l.ring = append(l.ring, ev)
	if len(l.ring) > supervisorEventRingSize {
		l.ring = l.ring[len(l.ring)-supervisorEventRingSize:]
	}
	w := l.writer
	l.mu.Unlock()

	if w != nil {
		if line, err := l.marsh.Marshal(ev); err == nil {
			_, _ = w.Write(append(line, '\n'))
		}
	}
	return ev
}

// EventsSince returns ring events with seq greater than lastSeq, ascending.
func (l *supervisorEventLog) EventsSince(lastSeq int64) []*opsv1.SupervisorEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]*opsv1.SupervisorEvent, 0, len(l.ring))
	for _, ev := range l.ring {
		if ev.Seq > lastSeq {
			out = append(out, ev)
		}
	}
	return out
}

// LatestSeq returns the newest assigned sequence number.
func (l *supervisorEventLog) LatestSeq() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq
}

// ReadTail reads at most maxBytes from the tail of the current log file,
// aligned to the next line start so the content holds whole JSON lines.
func (l *supervisorEventLog) ReadTail(maxBytes int64) (content []byte, fileName string, truncated bool, err error) {
	if l == nil || l.writer == nil {
		return nil, "", false, errors.New("supervisor log disabled")
	}
	if maxBytes <= 0 {
		maxBytes = 512 * 1024
	}
	path := l.writer.Filename
	f, err := os.Open(path)
	if err != nil {
		return nil, filepath.Base(path), false, err
	}
	defer func() { _ = f.Close() }()

	st, err := f.Stat()
	if err != nil {
		return nil, filepath.Base(path), false, err
	}
	size := st.Size()
	if size == 0 {
		return []byte{}, filepath.Base(path), false, nil
	}
	if size <= maxBytes {
		buf := make([]byte, size)
		if _, err = f.ReadAt(buf, 0); err != nil {
			return nil, filepath.Base(path), false, err
		}
		return buf, filepath.Base(path), false, nil
	}
	buf := make([]byte, maxBytes)
	if _, err = f.ReadAt(buf, size-maxBytes); err != nil {
		return nil, filepath.Base(path), false, err
	}
	// 对齐到下一个行首，保证下载内容都是完整 JSON 行。
	if idx := bytes.IndexByte(buf, '\n'); idx >= 0 && idx+1 < len(buf) {
		buf = buf[idx+1:]
	}
	return buf, filepath.Base(path), true, nil
}

// emitSupervisorEvent records one event for the process and stamps the
// process's last-event time (panel freshness). Safe to call without p.mu.
func (s *OpsServer) emitSupervisorEvent(p *managedProcess, ev *opsv1.SupervisorEvent) {
	if s.supLog == nil || ev == nil {
		return
	}
	s.supLog.emit(ev)
	if p == nil {
		return
	}
	p.mu.Lock()
	p.lastEventUnix = ev.TsUnix
	p.mu.Unlock()
}

// newSupervisorEvent seeds an event with the closed-set type and now timestamp.
func newSupervisorEvent(processName, event string) *opsv1.SupervisorEvent {
	return &opsv1.SupervisorEvent{
		TsUnix:  time.Now().Unix(),
		Process: processName,
		Event:   event,
	}
}
