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

// GovernDecision 是执行治理回调的判定（ARCH-024）。
type GovernDecision int

const (
	// GovernAllow 治理通过，继续派发。
	GovernAllow GovernDecision = iota
	// GovernApprovalRequired 政策要求审批：治理回调内已创建审批请求，
	// 调度不派发、run-log 记 approval_required。
	GovernApprovalRequired
	// GovernDenied 治理拒绝（禁用/未分配/权限不足/撤权）：不派发、
	// run-log 记 failed，err 携带拒绝原因。
	GovernDenied
)

// GovernHook 是可注入的执行治理回调（ARCH-024）：自动触发在派发前调用，
// 以调度创建者（s.Actor）为自动化主体逐次重新授权——撤权后不继续裸派发。
// 由 svc/cmd 布线注入 functioninvoke.Govern 的实现；nil（未注入）= 向后
// 兼容（不治理，行为与治理上线前一致）。
type GovernHook func(ctx context.Context, s *model.TaskSchedule) (GovernDecision, error)

// Store 依赖的最小模型接口。
type ScheduleStore interface {
	ListDue(ctx context.Context, now time.Time, limit int) ([]model.TaskSchedule, error)
	UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error
	// HasRunLog 判断触发槽是否已记录（幂等窗口）。
	HasRunLog(ctx context.Context, scheduleID uint, slot time.Time) (bool, error)
	// CreateRunLog 写触发记录；唯一索引冲突返回 false（并发兜底）。
	CreateRunLog(ctx context.Context, log *model.TaskScheduleRunLog) (bool, error)
	// GetRunLog 取指定槽位的触发记录（(nil, nil) = 不存在）。ARCH-023 租约
	// 判定读既有领取的 CreatedAt/Status。
	GetRunLog(ctx context.Context, scheduleID uint, slot time.Time) (*model.TaskScheduleRunLog, error)
	// UpdateRunLog 回填领取槽位后的派发结果（ARCH-023 先领取再派发）。
	UpdateRunLog(ctx context.Context, scheduleID uint, slot time.Time, taskRunID, status, message string) error
	// ListStaleClaims 列出 created_at < before 的 claimed 领取记录（ARCH-023
	// 跨槽崩溃恢复：旧槽遗留领取不在当前触发槽，由统一扫描解析）。
	ListStaleClaims(ctx context.Context, before time.Time, limit int) ([]model.TaskScheduleRunLog, error)
	// LastRunResult 取某 TaskRun 的终态（"" = 未终态/不存在）。
	LastRunStatus(ctx context.Context, taskRunID string) (string, error)
}

// Manager 是调度循环。
type Manager struct {
	store      ScheduleStore
	dispatch   Dispatcher
	local      LocalRunner
	govern     GovernHook
	interval   time.Duration
	nowFn      func() time.Time
	claimLease time.Duration // ARCH-023 领取租约：超过即视为领取者崩溃
	mu         sync.Mutex
	started    bool
	stopped    bool // Stop 已被调用（幂等守卫：二次调用不再 close stopCh）
	stopCh     chan struct{}
	stoppedCh  chan struct{}
	// lifecycleCtx is cancelled by Stop so a blocked tick (store/dispatch on
	// the lifecycle context) aborts instead of keeping the loop from exiting,
	// which would make the runServer defer StopScheduler hang past the 30s
	// shutdown budget (ARCH-017 R30-07).
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	// stopTimeout 是 Stop 等待循环退出的硬上限（0 = defaultStopTimeout）。
	// lifecycle ctx 只中断配合取消的调用；不配合取消的卡死（如驱动忽略
	// ctx 的库调用、钻机注入的 tick 钩子）仍会挡住 stoppedCh——Stop 必须
	// 有硬界，否则同步调用与 defer 兜底都无限等待（ARCH-017 补齐）。
	stopTimeout time.Duration
	// tickHook 在每轮 tick 起点调用（停机钻机注入卡死点用；生产恒 nil）。
	tickHook func()
}

// NewManager creates a scheduler manager. interval 为扫描周期（默认 30s）。
func NewManager(store ScheduleStore, dispatch Dispatcher) *Manager {
	return &Manager{
		store:      store,
		dispatch:   dispatch,
		interval:   30 * time.Second,
		nowFn:      time.Now,
		claimLease: 60 * time.Second,
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

// SetGovernHook 注入执行治理回调（ARCH-024）。nil 清除 = 向后兼容
// （不治理）。触发前读取，注入时序只须早于首个自动触发即可。
func (m *Manager) SetGovernHook(h GovernHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.govern = h
}

// SetInterval 覆盖扫描周期（测试用）。
func (m *Manager) SetInterval(d time.Duration) { m.interval = d }

// SetNow 覆盖时钟（测试用）。
func (m *Manager) SetNow(fn func() time.Time) { m.nowFn = fn }

// SetClaimLease 覆盖领取租约时长（测试用；缺省 60s）。
func (m *Manager) SetClaimLease(d time.Duration) { m.claimLease = d }

// SetStopTimeout 覆盖 Stop 等待循环退出的硬上限（测试用；缺省
// defaultStopTimeout）。须在 Start 前调用（stopTimeout 在 Stop 时读取，
// 晚于 Start 亦安全）。
func (m *Manager) SetStopTimeout(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopTimeout = d
}

// SetTickHook 注入每轮 tick 起点的前置钩子（停机钻机用；生产勿设）。
// 钩子在锁外调用——阻塞它即模拟「tick 卡死在 context.Background 库调用」
// 的 R30-07 场景（该卡死不观察 lifecycle ctx，靠 Stop 硬界兜底）。nil 清除。
func (m *Manager) SetTickHook(h func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickHook = h
}

// Start 启动调度循环（非阻塞）。
func (m *Manager) Start() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	// Lifecycle context: canceled on Stop so an in-progress tick using it is
	// aborted, keeping Stop bounded (ARCH-017 R30-07).
	m.lifecycleCtx, m.lifecycleCancel = context.WithCancel(context.Background())
	m.mu.Unlock()
	go m.loop()
}

// defaultStopTimeout 是 Stop 等待循环退出的缺省硬上限。lifecycle ctx 让
// 配合取消的调用及时中断；不配合取消的卡死（驱动忽略 ctx、钻机注入的
// tick 钩子）仍需硬界，否则同步调用与 runServer 的 defer 兜底都会无限
// 等待（ARCH-017 R30-07：外层 30s 返回后仍执行无界 defer StopScheduler）。
const defaultStopTimeout = 5 * time.Second

// Stop 停止调度循环并等待退出：先取消 lifecycle ctx（配合取消的阻塞调用
// 立即中断），再至多等待 stopTimeout（缺省 defaultStopTimeout）等循环退出。
// 幂等（ARCH-017：优雅停机在关池前显式调用 StopScheduler，同时保留 runServer
// 的 defer 兜底——二次调用必须安全）。超时返回时循环可能仍卡死在 tick 里：
// 停机继续推进（关池/冲刷由后续步骤完成），卡死 goroutine 随进程退出回收；
// tick 一旦返回，循环仍会观察已关闭的 stopCh 自行退出。
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	timeout := m.stopTimeout
	if timeout <= 0 {
		timeout = defaultStopTimeout
	}
	if m.stopped {
		m.mu.Unlock()
		m.waitStopped(timeout)
		return
	}
	m.stopped = true
	if m.lifecycleCancel != nil {
		m.lifecycleCancel()
	}
	m.mu.Unlock()
	close(m.stopCh)
	m.waitStopped(timeout)
}

// waitStopped 有界等待循环退出：正常路径 stoppedCh 关闭即返回；卡死 tick 下
// 到时返回（带日志），不阻塞调用方。
func (m *Manager) waitStopped(timeout time.Duration) {
	select {
	case <-m.stoppedCh:
	case <-time.After(timeout):
		slog.Warn("scheduler: stop timed out; a stuck tick may still be running",
			"timeout", timeout.String())
	}
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

// tick 处理一个扫描周期：到期触发 + 跨槽领取恢复（ARCH-023 R30-04）。
func (m *Manager) tick(now time.Time) {
	m.mu.Lock()
	h := m.tickHook
	m.mu.Unlock()
	if h != nil {
		h() // 锁外调用：钻机阻塞它即模拟不观察 lifecycle ctx 的卡死 tick
	}
	// 生命周期 context：Stop 会取消它，使卡住的 store/dispatch 调用中断，
	// 而非在 context.Background() 上无限阻塞——后者会让 Stop 等待
	// stoppedCh 无界，令 runServer 的 defer StopScheduler 在 30s 停机预算
	// 外仍挂住（ARCH-017 R30-07）。
	ctx := m.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	due, err := m.store.ListDue(ctx, now, 100)
	if err != nil {
		slog.Warn("scheduler: list due schedules failed", "error", err)
		return
	}
	for i := range due {
		m.triggerSchedule(ctx, &due[i], now)
	}
	m.reapStaleClaims(ctx, now)
}

// reapStaleClaims 跨槽扫描租约过期的 claimed（ARCH-023 R30-04 恢复缺口）：
// 领取者崩溃后遗留的旧槽位不在当前触发槽（triggerSchedule 只解析当前槽），
// 由这里按租约窗口统一列出并解析为 indeterminate——run-log 不再永久停在
// claimed，审计保留「派发结果未知」。绝不重派发（重发=对派发后中断赌二次
// 执行；业务最终副作用去重归 GAME-010）。活跃领取（租约内）不动。
func (m *Manager) reapStaleClaims(ctx context.Context, now time.Time) {
	stale, err := m.store.ListStaleClaims(ctx, now.Add(-m.claimLease), 100)
	if err != nil {
		slog.Warn("scheduler: list stale claims failed", "error", err)
		return
	}
	for i := range stale {
		rec := &stale[i]
		slog.Warn("scheduler: stale claim lease expired, mark indeterminate",
			"schedule", rec.ScheduleID, "slot", rec.Slot.Format(time.RFC3339),
			"age", now.Sub(rec.CreatedAt).String())
		_ = m.store.UpdateRunLog(ctx, rec.ScheduleID, rec.Slot, "",
			model.RunLogIndeterminate, "claim lease expired; dispatch outcome unknown (crash before writeback)")
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
	// 领取令牌。唯一冲突（claimed==false）说明另一实例已领取本槽——只推进
	// 下次触发时间、不派发。旧顺序（先 HasRunLog 判重→派发→后补记录且忽略
	// 冲突返回）在双 Manager 下双方都通过判重、双双派发、仅 1 条唯一记录。
	claimed, err := m.store.CreateRunLog(ctx, &model.TaskScheduleRunLog{
		ScheduleID: s.ID, Slot: slot, Status: "claimed",
	})
	if err != nil {
		// 基础设施错误：宁可不派发也不冒重复派发风险；不推进，下轮重试同槽。
		slog.Warn("scheduler: claim run slot failed, skip dispatch", "schedule", s.ID, "error", err)
		return
	}
	if !claimed {
		// 已被领取：他实例占住本槽，或领取者崩溃后的遗留。先解析陈旧领取
		// （租约过期 → 终态 indeterminate，派发结果未知），再推进下次触发时间。
		m.resolveStaleClaim(ctx, s, slot, now)
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
	// Kind=incident_report 是进程内生成器（非注册函数派发），不适用函数级
	// 执行治理——已在上分支返回。
	m.mu.Lock()
	local := m.local
	m.mu.Unlock()
	if s.Kind == model.ScheduleKindIncidentReport && local != nil {
		m.triggerLocal(ctx, local, s, slot, spec, now, failures)
		return
	}

	// 执行准入（ARCH-024 可注入治理服务）：自动触发以调度创建者
	// （s.Actor）为自动化主体逐次重新授权——撤权后不继续裸派发。未注入
	// 治理钩子（nil）向后兼容。Allow → 继续派发；ApprovalRequired /
	// Denied → 不派发、run-log 记录并推进下次触发时间。
	m.mu.Lock()
	govern := m.govern
	m.mu.Unlock()
	if govern != nil {
		decision, gerr := govern(ctx, s)
		switch decision {
		case GovernApprovalRequired:
			// 政策要求审批：治理回调已创建审批请求，不派发。
			slog.Info("scheduler: approval required, skip dispatch", "schedule", s.ID, "slot", slot.Format(time.RFC3339))
			_ = m.store.UpdateRunLog(ctx, s.ID, slot, "", "approval_required", "")
			m.advance(s, spec, now, nil)
			return
		case GovernDenied:
			msg := "governance denied"
			if gerr != nil {
				msg = "governance denied: " + gerr.Error()
			}
			slog.Warn("scheduler: governance denied, skip dispatch", "schedule", s.ID, "slot", slot.Format(time.RFC3339), "error", gerr)
			_ = m.store.UpdateRunLog(ctx, s.ID, slot, "", "failed", msg)
			m.advance(s, spec, now, nil)
			return
		default: // GovernAllow 继续派发
		}
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

// resolveStaleClaim 处理被占槽位的既有领取记录（ARCH-023 崩溃恢复）。
// 领取后超过租约仍停在 claimed 的，视为领取者崩溃——覆盖两类中断：
//   - 领取后/派发前中断：任务从未派发，槽位被解析为 indeterminate 并推进；
//   - 派发后/回写前中断：任务可能已派发但回写未落。
//
// 两类无法区分，R30-04 规定不得记为确定失败（failed）——indeterminate 是
// 唯一诚实的终态：「派发结果未知」，审计保留人工核验入口。
//
// 关键约束：不重派发。重发会让「派发后崩溃」的槽位重复执行（业务最终
// 副作用去重由 GAME-010 承担，执行层不赌二次运行）。活跃领取（未过租约，
// 他实例正在派发）与已终态记录（dispatched/failed/skipped，他实例已回填
// 或本槽本就结束）均不解析，只推进。
func (m *Manager) resolveStaleClaim(ctx context.Context, s *model.TaskSchedule, slot time.Time, now time.Time) {
	rec, err := m.store.GetRunLog(ctx, s.ID, slot)
	if err != nil {
		slog.Warn("scheduler: read existing run log failed", "schedule", s.ID, "error", err)
		return
	}
	if rec == nil || rec.Status != model.RunLogClaimed {
		return
	}
	age := now.Sub(rec.CreatedAt)
	if age <= m.claimLease {
		return
	}
	slog.Warn("scheduler: claim lease expired, mark indeterminate",
		"schedule", s.ID, "slot", slot.Format(time.RFC3339), "age", age.String())
	_ = m.store.UpdateRunLog(ctx, s.ID, slot, "", model.RunLogIndeterminate,
		"claim lease expired; dispatch outcome unknown (crash before writeback)")
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
