package agent

// 调度暂停状态持久化（agent-supervisor-design §10.2 暂停与恢复，暂停对象=调度）：
// 运维对某进程的 cron 调度意图是「维护窗口内不要触发」，与期望状态同源（supervisor
// 日志目录内落盘），跨 agent 重启保留。文件缺失/损坏 fail-open 空映射（缺省不暂停，
// 宁可不拦误触发，不因状态文件坏了拒绝装配），原子写 tmp+rename。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// schedulePauseStateFile 调度暂停状态落盘文件名（supervisor 日志目录内）。
const schedulePauseStateFile = "schedule_pause_state.json"

// schedulePauseStore name → 调度是否暂停（true=暂停触发）。缺记录视为未暂停；
// 文件缺失/损坏整体 fail-open 回空映射。
type schedulePauseStore struct {
	mu    sync.Mutex
	path  string
	pause map[string]bool
}

// newSchedulePauseStore 读取落盘暂停状态。目录解析与 supervisor 事件日志同源
// （SupervisorLogConfig.Dir，空回 logs/supervisor）。
func newSchedulePauseStore(cfg SupervisorLogConfig) *schedulePauseStore {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		dir = filepath.Join("logs", "supervisor")
	}
	s := &schedulePauseStore{
		path:  filepath.Join(dir, schedulePauseStateFile),
		pause: map[string]bool{},
	}
	if raw, err := os.ReadFile(s.path); err == nil {
		// 损坏文件 fail-open：空映射 + 告警，不阻断装配。
		if err := json.Unmarshal(raw, &s.pause); err != nil {
			s.pause = map[string]bool{}
			slog.Warn("managed process schedule pause state file corrupted; ignoring",
				"path", s.path, "error", err)
		}
	}
	return s
}

// Paused 返回某进程的调度是否暂停；ok=false 表示从无记录（视同未暂停）。
func (s *schedulePauseStore) Paused(name string) (paused, ok bool) {
	if s == nil {
		return false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.pause[name]
	return v, ok
}

// SetPaused 更新并落盘暂停状态。落盘失败仅告警不回错——暂停状态是持久性
// 提示，操作本身已成功，不因写盘失败回滚暂停/恢复。
func (s *schedulePauseStore) SetPaused(name string, paused bool) {
	if s == nil || strings.TrimSpace(name) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pause[name] = paused
	raw, err := json.Marshal(s.pause)
	if err != nil {
		slog.Warn("managed process schedule pause state marshal failed", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		slog.Warn("managed process schedule pause state dir create failed", "path", s.path, "error", err)
		return
	}
	// 原子写：临时文件 + rename，避免半截 JSON 被下次启动当损坏忽略。
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		slog.Warn("managed process schedule pause state write failed", "path", s.path, "error", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		slog.Warn("managed process schedule pause state rename failed", "path", s.path, "error", err)
	}
}

// String 调试面。
func (s *schedulePauseStore) String() string {
	if s == nil {
		return "<nil schedule pause state>"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("schedule_pause(%v)", s.pause)
}

// SetSchedulePause suspends or resumes the declared schedule firing for one
// managed process (§10.2 暂停与恢复，暂停对象=调度). 与手动 start/stop 同双闸
// （Enabled+AllowRestart）——暂停调度是远程运维意图，不改进程运行态；进程
// 未物化（无运行实例）也允许暂停（暂停意图只随 cron 生效）。
func (s *OpsServer) SetSchedulePause(ctx context.Context, req *opsv1.SchedulePauseRequest) (*opsv1.SchedulePauseResponse, error) {
	if !s.config.Enabled || !s.config.AllowRestart {
		return nil, fmt.Errorf("ops restart is not enabled")
	}
	name := strings.TrimSpace(req.GetProcessName())
	if name == "" {
		return nil, fmt.Errorf("process name is required")
	}
	s.mu.RLock()
	_, hasCfg := s.config.ManagedProcesses[name]
	s.mu.RUnlock()
	if !hasCfg {
		return nil, fmt.Errorf("process '%s' not found", name)
	}

	s.schedPause.SetPaused(name, req.GetSuspend())

	// 事件归属到实例时间戳（p 可空——未物化实例的暂停同样留痕）。
	s.mu.RLock()
	p, _ := s.processes[name]
	s.mu.RUnlock()
	event := supervisorEventSchedulePaused
	message := "schedule firing suspended"
	if !req.GetSuspend() {
		event = supervisorEventScheduleResumed
		message = "schedule firing resumed"
	}
	ev := newSupervisorEvent(name, event)
	ev.Message = message
	s.emitSupervisorEvent(p, ev)

	return &opsv1.SchedulePauseResponse{
		Success:   true,
		Message:   message,
		Suspended: req.GetSuspend(),
	}, nil
}

// isSchedulePaused 报告进程调度是否处于暂停态（schedPause 为 nil 视为未暂停）。
func (s *OpsServer) isSchedulePaused(name string) bool {
	if s.schedPause == nil {
		return false
	}
	paused, _ := s.schedPause.Paused(name)
	return paused
}
