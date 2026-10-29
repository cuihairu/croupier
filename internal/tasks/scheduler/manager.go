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

// LocalRunner 是 server-local 执行面（Kind=incident_report 的调度不经
// Dispatcher/Agent 派发链，报告生成器在进程内直接执行）。由 svc 布线
// 注入（internal/api/incident 的报告生成器实现）。
type LocalRunner interface {
	Run(ctx context.Context, s *model.TaskSchedule) error
}

// Store 依赖的最小模型接口。
type ScheduleStore interface {
	ListDue(ctx context.Context, now time.Time, limit int) ([]model.TaskSchedule, error)
	UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error
	// HasRunLog 判断触发槽是否已记录（幂等窗口）。
	HasRunLog(ctx context.Context, scheduleID uint, slot time.Time) (bool, error)
	// CreateRunLog 写触发记录；唯一索引冲突返回 false（并发兜底）。
	CreateRunLog(ctx context.Context, log *model.TaskScheduleRunLog) (bool, error)
	// UpdateRunLog 回填领取槽位后的派发结果（ARCH-023 先领取再派发）。
	UpdateRunLog(ctx context.Context, scheduleID uint, slot time.Time, taskRunID, status, message string) error
	// LastRunResult 取某 TaskRun 的终态（"" = 未终态/不存在）。
	LastRunStatus(ctx context.Context, taskRunID string) (string, error)
	// ReclaimStaleRunLog 崩溃恢复：仅当 (schedule_id, slot) 的领取行仍是
	// status='claimed' 且 created_at 早于 olderThan（租约过期、领取方已崩溃）
	// 时原子重领并返回 true；否则返回 false（他人持有 / 已终态 / 租约未过）。
	// 单条 UPDATE 的 CAS 语义保证并发恢复者仅一个命中。
	ReclaimStaleRunLog(ctx context.Context, scheduleID uint, slot time.Time, olderThan time.Time) (bool, error)
}

// Manager 是调度循环。
type Manager struct {
	store      ScheduleStore
	dispatch   Dispatcher
	local      LocalRunner
	interval   time.Duration
	claimLease time.Duration
	nowFn      func() time.Time
	mu         sync.Mutex
	started    bool
	stopCh     chan struct{}
	stoppedCh  chan struct{}
}

// NewManager creates a scheduler manager. interval 为扫描周期（默认 30s）；
// claimLease 为领取租约（默认 60s = 2×扫描周期）：领取行 created_at 早于
// 租约窗口且仍 status='claimed' 视为孤儿（领取方崩溃），可被原子重领恢复。
func NewManager(store ScheduleStore, dispatch Dispatcher) *Manager {
	return &Manager{
		store:      store,
		dispatch:   dispatch,
		interval:   30 * time.Second,
		claimLease: 60 * time.Second,
		nowFn:      time.Now,
		stopCh:     make(chan struct{}),
		stoppedCh:  make(chan struct{}),
	}
}

// SetLocalRunner 注入 server-local 执行面（Kind=incident_report 路由）。
// nil 清除。
func (m *Manager) SetLocalRunner(r LocalRunner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.local = r
}

// SetInterval 覆盖扫描周期（测试用）。
func (m *Manager) SetInterval(d time.Duration) { m.interval = d }

// SetClaimLease 覆盖领取租约（测试用）。租约越短崩溃恢复越快，但越小越
// 容易被一个仍在执行的长派发误判为孤儿（租约低估 → 双派发，游戏侧按
// GAME-010 去重兜底）。
func (m *Manager) SetClaimLease(d time.Duration) { m.claimLease = d }

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
	// ARCH-023 先领取再派发：CreateRunLog 的 (schedule_id, slot) 唯一索引就是
	// 领取令牌。唯一冲突（claimed==false）说明本槽已被领取——此时区分两种
	// 情况：活实例持有（租约未过 → 只推进不派发）与孤儿领取（领取方崩溃、
	// 租约已过 → 原子重领后继续派发，崩溃恢复）。旧顺序（先 HasRunLog 判重
	// →派发→后补记录且忽略冲突返回）在双 Manager 下双方都通过判重、双双
	// 派发、仅 1 条唯一记录。
	claimed, err := m.store.CreateRunLog(ctx, &model.TaskScheduleRunLog{
		ScheduleID: s.ID, Slot: slot, Status: "claimed",
	})
	if err != nil {
		// 基础设施错误：宁可不派发也不冒重复派发风险；不推进，下轮重试同槽。
		slog.Warn("scheduler: claim run slot failed, skip dispatch", "schedule", s.ID, "error", err)
		return
	}
	if !claimed {
		// 领取冲突：尝试崩溃恢复重领。ReclaimStaleRunLog 是单条 UPDATE 的
		// CAS——仅当行仍 status='claimed' 且 created_at 早于租约窗口时命中，
		// 并把租约重置为 now。并发恢复者仅一个命中，其余因 created_at 已被
		// 刷新而失配，不会同槽双恢复。
		reclaimed, reclaimErr := m.store.ReclaimStaleRunLog(ctx, s.ID, slot, now.Add(-m.claimLease))
		if reclaimErr != nil {
			// 恢复判定失败：保守跳过（不冒双派发风险），下轮再试。
			slog.Warn("scheduler: stale claim reap failed, skip dispatch", "schedule", s.ID, "error", reclaimErr)
			return
		}
		if !reclaimed {
			// 活领取或已终态：只推进下次触发时间，不派发。
			m.advance(s, spec, now, nil)
			return
		}
		// 重领成功：本实例接管该槽，落入下方派发路径（崩溃恢复派发）。
		slog.Info("scheduler: recovered stale claim, redispatch slot", "schedule", s.ID, "slot", slot)
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
		_ = m.store.UpdateRunLog(ctx, s.ID, slot, "", "skipped", "dead_letter: consecutive failures reached limit")
		_ = m.store.UpdateSchedule(ctx, s.ID, map[string]interface{}{
			"status":               model.ScheduleStatusDeadLetter,
			"consecutive_failures": failures,
			"next_triggered_at":    nil,
		})
		return
	}

	// server-local 路由：报告生成器进程内直接执行（不经 Dispatcher，
	// 无 TaskRun）。Kind 不匹配或未注入 LocalRunner 时落既有派发链。
	m.mu.Lock()
	local := m.local
	m.mu.Unlock()
	if s.Kind == model.ScheduleKindIncidentReport && local != nil {
		m.triggerLocal(ctx, local, s, slot, spec, now, failures)
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
	meta["gameId"] = s.GameID
	meta["env"] = s.Env
	meta["scheduleId"] = fmt.Sprintf("%d", s.ID)
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

	// 回填领取后的派发结果（claim 已在触发前写入）。
	_ = m.store.UpdateRunLog(ctx, s.ID, slot, runID, trigStatus, trigMsg)

	updates := map[string]interface{}{
		"last_triggered_at":    now,
		"last_run_id":          runID,
		"consecutive_failures": failures,
	}
	m.advanceWith(s, spec, now, updates)
	_ = m.store.UpdateSchedule(ctx, s.ID, updates)
}

// triggerLocal 执行 server-local 调度（Kind=incident_report）：同步调用
// LocalRunner——成功清零失败计数、失败 +1；run-log 回填 dispatched/failed
// （无 TaskRunID——不经异步任务链）。领取已在 triggerSchedule 完成。
func (m *Manager) triggerLocal(ctx context.Context, local LocalRunner, s *model.TaskSchedule, slot time.Time, spec *CronSpec, now time.Time, failures int) {
	err := local.Run(ctx, s)
	status, msg := "dispatched", ""
	if err != nil {
		status, msg = "failed", err.Error()
		failures++
		slog.Warn("scheduler: local run failed", "schedule", s.ID, "error", err)
	} else {
		failures = 0
	}
	_ = m.store.UpdateRunLog(ctx, s.ID, slot, "", status, msg)
	updates := map[string]interface{}{
		"last_triggered_at":    now,
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
