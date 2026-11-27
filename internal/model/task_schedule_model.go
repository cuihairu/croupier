package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// ScheduleScopeResolver 按 (gameID, env) 解析 per-game 数据库的 ctx 覆盖
// （与 tasks.ScopeResolver 同型：消费方定义窄接口，由 svc/cmd 侧把
// router 适配注入，model 不 import internal/db/router）。AUD-087：task_runs
// 是 game 模型（model.GameModels）——multiGame 模式下 live 在 schedule
// 所属作用域的 game 库，meta 库没有该表，终态回查必须按作用域路由。
type ScheduleScopeResolver interface {
	// ScopeContext 返回携带该作用域 per-game DB 覆盖的 ctx（dbctx 约定）。
	// 解析失败必须以 error 透传：终态回查的「查库失败」要与「记录不存在 /
	// 未终态」可区分——静默回退 meta 连接会复活 AUD-087 缺陷（multiGame 下
	// 恒判「无记录」，失败计数永不推进/清零）。
	ScopeContext(ctx context.Context, gameID, env string) (context.Context, error)
}

// TaskScheduleModel 提供 cron 调度定义的 CRUD 与到期扫描。
type TaskScheduleModel struct {
	db *gorm.DB
	// scopes 按 schedule 作用域解析 game 库（multiGame 由 svc 注入；
	// nil = 单库模式，task_runs 与调度定义同库，无需路由）。
	scopes ScheduleScopeResolver
}

func NewTaskScheduleModel(db *gorm.DB) *TaskScheduleModel {
	return &TaskScheduleModel{db: db}
}

// SetScopeResolver 注入作用域解析（multiGame 模式在 svc 布线 Router 适配）。
// nil 清除 = 单库模式行为。
func (m *TaskScheduleModel) SetScopeResolver(r ScheduleScopeResolver) {
	m.scopes = r
}

// runStatusDB 解析终态回查的目标连接：有作用域（gameID 非空）且已注入
// 解析器时路由 game 库，否则回退模型自身连接。单库模式（无解析器）两者
// 本是同一库，行为不变。
func (m *TaskScheduleModel) runStatusDB(ctx context.Context, gameID, env string) (*gorm.DB, error) {
	if m.scopes == nil || gameID == "" {
		return m.db, nil
	}
	scoped, err := m.scopes.ScopeContext(ctx, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("resolve schedule scope game %q env %q: %w", gameID, env, err)
	}
	return dbctx.Resolve(scoped, m.db), nil
}

// ValidateScheduleInput 校验并归一化创建/更新输入。
func ValidateScheduleInput(name, cronExpr, gameID, env, functionID string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("名称不能为空")
	}
	if strings.TrimSpace(cronExpr) == "" {
		return errors.New("cron 表达式不能为空")
	}
	if strings.TrimSpace(gameID) == "" || strings.TrimSpace(env) == "" {
		return errors.New("gameId/env 不能为空")
	}
	if strings.TrimSpace(functionID) == "" {
		return errors.New("functionId 不能为空")
	}
	return nil
}

type CreateScheduleInput struct {
	Name       string
	CronExpr   string
	GameID     string
	Env        string
	FunctionID string
	Payload    JSON
	Metadata   JSON
	MaxFailed  int
	Actor      string
	// Kind 触发形态（ScheduleKindFunction/ScheduleKindIncidentReport）；
	// 空 = function（默认派发链）。
	Kind string
	// FirstNext 首次触发时间（AUD-088）。Create 恒产出 active 行，而 ListDue
	// 只扫 next_triggered_at IS NOT NULL——首触时间为 NULL 的 active 行是
	// 永不触发的死行。该值由调用方按 cron 预计算（model 不 import scheduler，
	// 防环，先例：SeedIncidentReportSchedules 的 nextAfter 回调），并与行同
	// 一次 INSERT 落库：拆成 insert+update 会在 update 失败时留下死行且接口
	// 仍报成功。nil = 旧形态（行落库后首触时间为 NULL，由调用方自行回填；
	// 存量死行用 RepairMissingNextTrigger 盘点收敛）。
	FirstNext *time.Time
}

func (m *TaskScheduleModel) Create(ctx context.Context, in CreateScheduleInput) (*TaskSchedule, error) {
	if err := ValidateScheduleInput(in.Name, in.CronExpr, in.GameID, in.Env, in.FunctionID); err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = ScheduleKindFunction
	}
	s := &TaskSchedule{
		Name:          strings.TrimSpace(in.Name),
		CronExpr:      strings.TrimSpace(in.CronExpr),
		GameID:        strings.TrimSpace(in.GameID),
		Env:           strings.TrimSpace(in.Env),
		FunctionID:    strings.TrimSpace(in.FunctionID),
		Payload:       in.Payload,
		Metadata:      in.Metadata,
		Status:        ScheduleStatusActive,
		Kind:          kind,
		MaxFailedRuns: in.MaxFailed,
		Actor:         in.Actor,
		// AUD-088：首触时间与行同一次 INSERT 落库，create 不再依赖后续 update。
		NextTriggeredAt: in.FirstNext,
	}
	if s.MaxFailedRuns <= 0 {
		s.MaxFailedRuns = 5
	}
	if err := m.db.WithContext(ctx).Create(s).Error; err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	return s, nil
}

func (m *TaskScheduleModel) FindByID(ctx context.Context, id uint) (*TaskSchedule, error) {
	var s TaskSchedule
	if err := m.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

type ListSchedulesOptions struct {
	Page     int
	PageSize int
	GameID   string
	Env      string
	Status   string
}

func (m *TaskScheduleModel) List(ctx context.Context, opts ListSchedulesOptions) ([]TaskSchedule, int64, error) {
	q := m.db.WithContext(ctx).Model(&TaskSchedule{})
	if opts.GameID != "" {
		q = q.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		q = q.Where("env = ?", opts.Env)
	}
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if opts.Page > 0 && opts.PageSize > 0 {
		q = q.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}
	var out []TaskSchedule
	err := q.Order("id DESC").Find(&out).Error
	return out, total, err
}

// UpdateSchedule 更新字段（调度器与 API 共用）。
func (m *TaskScheduleModel) UpdateSchedule(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&TaskSchedule{}).Where("id = ?", id).Updates(updates).Error
}

// SetStatus 切换状态；从 dead_letter 恢复时清零失败计数并重算下次触发。
func (m *TaskScheduleModel) SetStatus(ctx context.Context, id uint, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == ScheduleStatusActive {
		updates["consecutive_failures"] = 0
	}
	return m.UpdateSchedule(ctx, id, updates)
}

// Delete 软删除。
func (m *TaskScheduleModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&TaskSchedule{}, id).Error
}

// ListDue 返回到期的 active 计划（调度循环扫描条件）。next_triggered_at
// IS NOT NULL 是扫描条件的一部分：active 且首触时间为 NULL 的行扫不出来
// （AUD-088）。该形态自创建起即被 Create 的 FirstNext 阻断；存量死行由
// RepairMissingNextTrigger 盘点收敛——这里不得放宽成 IS NULL 也触发，否则
// 一次性收敛会把积压死行集中派发。
func (m *TaskScheduleModel) ListDue(ctx context.Context, now time.Time, limit int) ([]TaskSchedule, error) {
	var out []TaskSchedule
	q := m.db.WithContext(ctx).
		Where("status = ? AND next_triggered_at IS NOT NULL AND next_triggered_at <= ?", ScheduleStatusActive, now)
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&out).Error
	return out, err
}

// RepairMissingNextTriggerResult 是存量死行（active 且 next_triggered_at 为
// NULL——ListDue 扫不出来，永不触发）的盘点结果。
type RepairMissingNextTriggerResult struct {
	// Repaired 已回填首次触发时间的行。
	Repaired []TaskSchedule
	// Unrepairable 无法回填的行（cron 非法或回调放弃），保持原样待人工处置。
	Unrepairable []TaskSchedule
}

// RepairMissingNextTrigger 盘点并收敛存量死行：active 且 next_triggered_at
// 为 NULL 的行 ListDue 永远扫不出（AUD-088）。逐行经 nextAfter 按 cron 重算
// 首触时间后回填；回填 UPDATE 带 status=active AND next_triggered_at IS NULL
// 再校验（窗口内被并发暂停/回填的行 RowsAffected=0，按竞争丢弃不算 repaired）。
// paused（暂停态清空首触时间是设计行为）与 dead_letter（恢复走 SetStatus 重算）
// 不在扫描范围。nextAfter 由调用方注入（svc 侧经 scheduler.ParseCron 实现——
// model 不 import scheduler，防环，先例 SeedIncidentReportSchedules）。
// 幂等：收敛完成后再次调用两个清单均为空（除仍非法的 cron 行）。
func (m *TaskScheduleModel) RepairMissingNextTrigger(ctx context.Context, nextAfter func(cronExpr string) (time.Time, bool)) (*RepairMissingNextTriggerResult, error) {
	var dead []TaskSchedule
	if err := m.db.WithContext(ctx).
		Where("status = ? AND next_triggered_at IS NULL", ScheduleStatusActive).
		Order("id ASC").
		Find(&dead).Error; err != nil {
		return nil, err
	}
	res := &RepairMissingNextTriggerResult{}
	for i := range dead {
		row := dead[i]
		if nextAfter == nil {
			res.Unrepairable = append(res.Unrepairable, row)
			continue
		}
		next, ok := nextAfter(row.CronExpr)
		if !ok {
			res.Unrepairable = append(res.Unrepairable, row)
			continue
		}
		tx := m.db.WithContext(ctx).Model(&TaskSchedule{}).
			Where("id = ? AND status = ? AND next_triggered_at IS NULL", row.ID, ScheduleStatusActive).
			Update("next_triggered_at", next)
		if tx.Error != nil {
			return res, tx.Error
		}
		if tx.RowsAffected == 0 {
			continue // 竞争：并发已暂停或已被他实例回填
		}
		row.NextTriggeredAt = &next
		res.Repaired = append(res.Repaired, row)
	}
	return res, nil
}

// HasRunLog 判断触发槽是否已记录。
func (m *TaskScheduleModel) HasRunLog(ctx context.Context, scheduleID uint, slot time.Time) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&TaskScheduleRunLog{}).
		Where("schedule_id = ? AND slot = ?", scheduleID, slot).
		Count(&count).Error
	return count > 0, err
}

// CreateRunLog 写触发记录。schedule_id+slot 唯一索引冲突时返回 false。
func (m *TaskScheduleModel) CreateRunLog(ctx context.Context, log *TaskScheduleRunLog) (bool, error) {
	err := m.db.WithContext(ctx).Create(log).Error
	if err != nil {
		if isDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetRunLog 取指定槽位的触发记录。返回 (nil, nil) 表示不存在。
// ARCH-023 领取租约判定需要读既有领取的 CreatedAt 与 Status。
func (m *TaskScheduleModel) GetRunLog(ctx context.Context, scheduleID uint, slot time.Time) (*TaskScheduleRunLog, error) {
	var log TaskScheduleRunLog
	err := m.db.WithContext(ctx).
		Where("schedule_id = ? AND slot = ?", scheduleID, slot).
		First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// UpdateRunLog 回填领取槽位后的派发结果（ARCH-023 先领取再派发：领取行
// 先占住唯一索引，派发完成后把 status/task_run_id/message 落为终值）。
//
// WHERE 带 status=claimed 再校验（R30-05）：领取→回填窗口内另一实例可能
// 已完成派发并回写 dispatched（或本槽已终态），若只按 schedule_id+slot 更新
// 会把真实终值覆写成 indeterminate。仅当行仍为 claimed 时才回填，否则 no-op
// 保留已落终值。
func (m *TaskScheduleModel) UpdateRunLog(ctx context.Context, scheduleID uint, slot time.Time, taskRunID, status, message string) error {
	return m.db.WithContext(ctx).Model(&TaskScheduleRunLog{}).
		Where("schedule_id = ? AND slot = ? AND status = ?", scheduleID, slot, RunLogClaimed).
		Updates(map[string]interface{}{
			"task_run_id": taskRunID,
			"status":      status,
			"message":     message,
		}).Error
}

// ListStaleClaimsAfter 返回 created_at < before、status=claimed 且
// id > cursor 的领取记录（ARCH-023 R30-04 跨槽崩溃恢复）。领取者崩溃后
// 遗留的旧槽位不在当前触发槽，triggerSchedule 的 resolveStaleClaim 只读
// 当前槽，无法看到——由调度循环的统一扫描按租约窗口列出，再解析为
// indeterminate。cursor 分页锚点用主键 id 而非 created_at：created_at 非
// 唯一，同时间戳多行 + 「>= cursor」会反复取同页；按 id ASC 保证全量收敛。
func (m *TaskScheduleModel) ListStaleClaimsAfter(ctx context.Context, before time.Time, cursor uint, limit int) ([]TaskScheduleRunLog, error) {
	var out []TaskScheduleRunLog
	q := m.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", RunLogClaimed, before)
	if cursor > 0 {
		q = q.Where("id > ?", cursor)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	q = q.Order("id ASC")
	err := q.Find(&out).Error
	return out, err
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key")
}

// LastRunStatus 返回 TaskRun 终态；未终态或不存在返回 ""。AUD-087：查询按
// schedule 所属 (gameID, env) 路由——task_runs 在 game 库（model.GameModels），
// multiGame 模式下 meta 库没有该表，按自身（meta）连接查会恒判「不存在」。
// 三种结果可区分：记录不存在与未终态均返回 ("", nil)（计数保持不动的中性
// 语义），作用域解析失败与查库错误返回 error，调用方不得吞掉。
func (m *TaskScheduleModel) LastRunStatus(ctx context.Context, gameID, env, taskRunID string) (string, error) {
	if taskRunID == "" {
		return "", nil
	}
	db, err := m.runStatusDB(ctx, gameID, env)
	if err != nil {
		return "", err
	}
	var run TaskRun
	err = db.WithContext(ctx).Where("task_id = ?", taskRunID).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	switch run.Status {
	case "succeeded", "failed", "cancelled", "timed_out":
		return run.Status, nil
	default:
		return "", nil
	}
}

// ListRunLogs 分页列出触发历史。
func (m *TaskScheduleModel) ListRunLogs(ctx context.Context, scheduleID uint, page, pageSize int) ([]TaskScheduleRunLog, int64, error) {
	q := m.db.WithContext(ctx).Model(&TaskScheduleRunLog{}).Where("schedule_id = ?", scheduleID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && pageSize > 0 {
		q = q.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	var out []TaskScheduleRunLog
	err := q.Order("id DESC").Find(&out).Error
	return out, total, err
}
