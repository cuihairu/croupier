package agent

// 受管进程管理状态持久化（agent-supervisor-design §10.3 管理状态面首块）：
// 运维的手动 start/stop 是**期望状态**，必须与运行状态分开记——此前只有
// 进程退出事件环与内存态，运维手动停掉的进程在 agent 重启后被开机自启
// 逻辑（AutoRestart=true 即拉起）误启。落盘与 supervisor 事件日志同目录，
// agent 重启后开机自启先查期望状态。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// desiredStateFile 期望状态落盘文件名（supervisor 日志目录内）。
const desiredStateFile = "desired_state.json"

// desiredStateStore name → 期望是否运行（true=期望 running）。缺记录视为
// 期望 running（对齐既有开机自启语义，行为零漂移）；文件缺失/损坏整体
// fail-open 回空映射（宁可不拦误启，不因状态文件坏了拒绝起进程）。
type desiredStateStore struct {
	mu      sync.Mutex
	path    string
	desired map[string]bool
}

// newDesiredStateStore 读取落盘期望状态。目录解析与 supervisor 事件日志
// 同源（SupervisorLogConfig.Dir，空回 logs/supervisor）。
func newDesiredStateStore(cfg SupervisorLogConfig) *desiredStateStore {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		dir = filepath.Join("logs", "supervisor")
	}
	s := &desiredStateStore{
		path:    filepath.Join(dir, desiredStateFile),
		desired: map[string]bool{},
	}
	if raw, err := os.ReadFile(s.path); err == nil {
		// 损坏文件 fail-open：空映射 + 告警，不阻断装配。
		if err := json.Unmarshal(raw, &s.desired); err != nil {
			s.desired = map[string]bool{}
			slog.Warn("managed process desired state file corrupted; ignoring",
				"path", s.path, "error", err)
		}
	}
	return s
}

// Running 返回某进程的期望运行态；ok=false 表示从无记录（视同期望 running）。
func (s *desiredStateStore) Running(name string) (running, ok bool) {
	if s == nil {
		return false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.desired[name]
	return v, ok
}

// SetRunning 更新并落盘期望状态。落盘失败仅告警不回错——期望状态是持久
// 性提示，操作本身已成功，不因写盘失败回滚 start/stop。
func (s *desiredStateStore) SetRunning(name string, running bool) {
	if s == nil || strings.TrimSpace(name) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.desired[name] = running
	raw, err := json.Marshal(s.desired)
	if err != nil {
		slog.Warn("managed process desired state marshal failed", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		slog.Warn("managed process desired state dir create failed", "path", s.path, "error", err)
		return
	}
	// 原子写：临时文件 + rename，避免半截 JSON 被下次启动当损坏忽略。
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		slog.Warn("managed process desired state write failed", "path", s.path, "error", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		slog.Warn("managed process desired state rename failed", "path", s.path, "error", err)
	}
}

// bootShouldStart 开机自启判定：期望状态无记录 → 沿既有语义（AutoRestart
// 即拉起）；有记录且期望 stopped → 不误启（运维主动停止的意图跨重启保留，
// supervisor-design §10.2「区分程序崩溃与主动停止」）。
func (s *desiredStateStore) bootShouldStart(name string) bool {
	if running, ok := s.Running(name); ok {
		return running
	}
	return true
}

// String 调试面。
func (s *desiredStateStore) String() string {
	if s == nil {
		return "<nil desired state>"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("desired_state(%v)", s.desired)
}
