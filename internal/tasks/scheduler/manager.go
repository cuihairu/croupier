package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// Dispatcher 是触发时要调用的派发接口（由 dispatch 层实现，
// 与 task.TaskRuntime.StartTask 同构，便于测试替身）。
type Dispatcher interface {
	StartTask(ctx context.Context, req *sdkv1.InvokeRequest) (*sdkv1.StartTaskResponse, error)
}

// Store 依赖的最小模型接口。
type ScheduleStore interface {
	ListDue(ctx context.Context, now time.Time, limit int) ([]model.TaskSchedule, error)
	UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error
	// HasRunLog 判断触发槽是否已记录（幂等窗口）。
	HasRunLog(ctx context.Context, scheduleID uint, slot time.Time) (bool, error)
	// CreateRunLog 写触发记录；唯一索引冲突返回 false（并发兜底）。
	CreateRunLog(ctx context.Context, log *model.TaskScheduleRunLog) (bool, error)
	// LastRunResult 取某 TaskRun 的终态（"" = 未终态/不存在）。
	LastRunStatus(ctx context.Context, taskRunID string) (string, error)
}

// Manager 是调度循环。
type Manager struct {
	store     ScheduleStore
	dispatch  Dispatcher
	interval  time.Duration
	nowFn     func() time.Time
	mu        sync.Mutex
	started   bool
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewManager creates a scheduler manager. interval 为扫描周期（默认 30s）。
func NewManager(store ScheduleStore, dispatch Dispatcher) *Manager {
	return &Manager{
		store:     store,
		dispatch:  dispatch,
		interval:  30 * time.Second,
		nowFn:     time.Now,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// SetInterval 覆盖扫描周期（测试用）。
func (m *Manager) SetInterval(d time.Duration) { m.interval = d }

// SetNow 覆盖时钟（测试用）。
func (m *Manager) SetNow(fn func() time.Time) { m.nowFn = fn }

// Start 启动调度循环（非阻塞）。
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	go m.loop()
}

// Stop 停止调度循环并等待退出。
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	close(m.stopCh)
	<-m.stoppedCh
}

func (m *Manager) loop() {
	defer close(m.stoppedCh)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.tick(m.nowFn())
		}
	}
}

// tick 处理一个扫描周期：到期触发 + 失败计数推进。
func (m *Manager) tick(now time.Time) {
	ctx := context.Background()
	due, err := m.store.ListDue(ctx, now, 100)
	if err != nil {
		slog.Warn("scheduler: list due schedules failed", "error", err)
		return
	}
	for i := range due {
		m.triggerSchedule(ctx, &due[i], now)
	}
}

// triggerSchedule 触发单条计划：幂等判重 → 派发 → 更新状态与下次时间。
func (m *Manager) triggerSchedule(ctx context.Context, s *model.TaskSchedule, now time.Time) {
	spec, err := ParseCron(s.CronExpr)
	if err != nil {
		// 定义损坏：直接进 dead_letter 防止每周期空转报错。
		slog.Error("scheduler: invalid cron, moving to dead_letter", "schedule", s.ID, "expr", s.CronExpr)
		_ = m.store.UpdateSchedule(ctx, s.ID, map[string]interface{}{
			"status": model.ScheduleStatusDeadLetter, "message": "invalid cron: " + err.Error(),
			"next_triggered_at": nil,
		})
		return
	}

	slot := Slot(now)
	// 幂等窗口：本触发槽已处理过（多实例/重启重叠场景）。
	if done, _ := m.store.HasRunLog(ctx, s.ID, slot); done {
		m.advance(s, spec, now, nil)
		return
	}

	// 触发前回查上一次执行的终态，推进连续失败计数。
	failures := s.ConsecutiveFailures
	if s.LastRunID != "" {
		if status, err := m.store.LastRunStatus(ctx, s.LastRunID); err == nil && isFailure(status) {
			failures++
		} else if err == nil && isSuccess(status) {
			failures = 0
		}
	}

	maxFailed := s.MaxFailedRuns
	if maxFailed <= 0 {
		maxFailed = 5
	}
	if failures >= maxFailed {
		slog.Warn("scheduler: schedule exceeded max failures, dead_letter", "schedule", s.ID, "failures", failures)
		_, _ = m.store.CreateRunLog(ctx, &model.TaskScheduleRunLog{
			ScheduleID: s.ID, Slot: slot, Status: "skipped", Message: "dead_letter: consecutive failures reached limit",
		})
		_ = m.store.UpdateSchedule(ctx, s.ID, map[string]interface{}{
			"status":               model.ScheduleStatusDeadLetter,
			"consecutive_failures": failures,
			"next_triggered_at":    nil,
		})
		return
	}

	// 派发 TaskRun（scope 经 metadata 传递，与 task.Start 同一约定）。
	var meta map[string]string
	if len(s.Metadata) > 0 {
		_ = json.Unmarshal(s.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]string{}
	}
	meta["game_id"] = s.GameID
	meta["env"] = s.Env
	meta["schedule_id"] = fmt.Sprintf("%d", s.ID)
	req := &sdkv1.InvokeRequest{
		FunctionId: s.FunctionID,
		Payload:    []byte(s.Payload.String()),
		Metadata:   meta,
	}

	runID := ""
	trigStatus := "dispatched"
	trigMsg := ""
	resp, err := m.dispatch.StartTask(ctx, req)
	if err != nil {
		trigStatus = "failed"
		trigMsg = err.Error()
		slog.Warn("scheduler: dispatch failed", "schedule", s.ID, "error", err)
	} else if resp != nil {
		runID = resp.GetTaskId()
	}

	_, _ = m.store.CreateRunLog(ctx, &model.TaskScheduleRunLog{
		ScheduleID: s.ID, Slot: slot, TaskRunID: runID, Status: trigStatus, Message: trigMsg,
	})

	updates := map[string]interface{}{
		"last_triggered_at":    now,
		"last_run_id":          runID,
		"consecutive_failures": failures,
	}
	m.advanceWith(s, spec, now, updates)
	_ = m.store.UpdateSchedule(ctx, s.ID, updates)
}

// advance 只推进下次触发时间（幂等命中路径）。
func (m *Manager) advance(s *model.TaskSchedule, spec *CronSpec, now time.Time, updates map[string]interface{}) {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	m.advanceWith(s, spec, now, updates)
	_ = m.store.UpdateSchedule(context.Background(), s.ID, updates)
}

func (m *Manager) advanceWith(s *model.TaskSchedule, spec *CronSpec, now time.Time, updates map[string]interface{}) {
	next := spec.Next(now)
	updates["next_triggered_at"] = next
}

func isFailure(status string) bool {
	return status == "failed" || status == "timed_out"
}

func isSuccess(status string) bool {
	return status == "succeeded"
}
