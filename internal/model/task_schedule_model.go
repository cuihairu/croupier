package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TaskScheduleModel 提供 cron 调度定义的 CRUD 与到期扫描。
type TaskScheduleModel struct {
	db *gorm.DB
}

func NewTaskScheduleModel(db *gorm.DB) *TaskScheduleModel {
	return &TaskScheduleModel{db: db}
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
	Payload    datatypes.JSON
	Metadata   datatypes.JSON
	MaxFailed  int
	Actor      string
}

func (m *TaskScheduleModel) Create(ctx context.Context, in CreateScheduleInput) (*TaskSchedule, error) {
	if err := ValidateScheduleInput(in.Name, in.CronExpr, in.GameID, in.Env, in.FunctionID); err != nil {
		return nil, err
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
		MaxFailedRuns: in.MaxFailed,
		Actor:         in.Actor,
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

// ListDue 返回到期的 active 计划（调度循环扫描条件）。
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

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key")
}

// LastRunStatus 返回 TaskRun 终态；未终态或不存在返回 ""。
func (m *TaskScheduleModel) LastRunStatus(ctx context.Context, taskRunID string) (string, error) {
	if taskRunID == "" {
		return "", nil
	}
	var run TaskRun
	err := m.db.WithContext(ctx).Where("task_id = ?", taskRunID).First(&run).Error
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
