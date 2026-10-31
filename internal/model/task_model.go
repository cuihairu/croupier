package model

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// OptionCount 服务端聚合选项的通用行形态（#23/#33/#34 族过滤下拉）。
// Scan 目标：`SELECT <col> AS name, COUNT(*) AS count ... GROUP BY <col>`。
type OptionCount struct {
	Name  string
	Count int64
}

type ListTasksOptions struct {
	PaginationOptions
	FunctionID string
	Status     string
	GameID     string
	Env        string
	// Actor 按操作者过滤（OPEN-ISSUES #23：/tasks 的 actor 参数此前未接线，
	// 前端发了参数被静默忽略，页面靠客户端兜底过滤——分页下漏数据）。
	Actor string
}

type TaskRunModel struct {
	db *gorm.DB
}

func NewTaskRunModel(db *gorm.DB) *TaskRunModel {
	return &TaskRunModel{db: db}
}

func (m *TaskRunModel) Create(ctx context.Context, task *TaskRun) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(task).Error
}

func (m *TaskRunModel) FindByTaskID(ctx context.Context, taskID string) (*TaskRun, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var task TaskRun
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (m *TaskRunModel) UpdateByTaskID(ctx context.Context, taskID string, updates map[string]interface{}) error {
	if strings.TrimSpace(taskID) == "" {
		return errors.New("task id required")
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&TaskRun{}).Where("task_id = ?", taskID).Updates(updates).Error
}

// UpdateByTaskIDIfStatusNotIn applies an update only while the task is not in
// one of the supplied statuses. It returns false when a concurrent terminal
// transition has already won, allowing callers to ignore late lifecycle
// events without rolling the task back to an intermediate state.
func (m *TaskRunModel) UpdateByTaskIDIfStatusNotIn(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}) (bool, error) {
	if strings.TrimSpace(taskID) == "" {
		return false, errors.New("task id required")
	}
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&TaskRun{}).Where("task_id = ?", taskID)
	if len(blockedStatuses) > 0 {
		query = query.Where("(status IS NULL OR status NOT IN ?)", blockedStatuses)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// RecordTaskEventTransactional 在单一数据库事务内执行带 guard 的状态更新与
// 事件追加，保证 task_runs 与 task_events 持久一致（R30-03 / ARCH-015）：
// 此前 handleTaskEvent 先更新 task_runs 再写 task_events、无共同事务，事件
// 写失败后重试被终态 guard 直接 ACK，缺失事件永不补写。updated=false 表示
// 终态 guard 拦截（同 UpdateByTaskIDIfStatusNotIn 语义，事务内不追加事件，
// 整体提交空操作）。
func (m *TaskRunModel) RecordTaskEventTransactional(ctx context.Context, taskID string, blockedStatuses []string, updates map[string]interface{}, event *TaskEvent) (bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, errors.New("task id required")
	}
	if event == nil {
		return false, errors.New("task event required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if len(event.Payload) == 0 {
		event.Payload = []byte("null")
	}
	event.TaskID = taskID
	updated := false
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&TaskRun{}).Where("task_id = ?", taskID)
		if len(blockedStatuses) > 0 {
			query = query.Where("(status IS NULL OR status NOT IN ?)", blockedStatuses)
		}
		result := query.Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil // 终态 guard 拦截：不追加事件
		}
		updated = true
		// seq 计算与追加须与状态更新同事务（NextSeq 语义：MAX(seq)+1，
		// 首条为 1）；失败随事务整体回滚，不产生部分写入。
		var latest TaskEvent
		err := tx.Where("task_id = ?", taskID).Order("seq DESC").First(&latest).Error
		switch {
		case err == nil:
			event.Seq = latest.Seq + 1
		case errors.Is(err, gorm.ErrRecordNotFound):
			event.Seq = 1
		default:
			return err
		}
		return tx.Create(event).Error
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

// ReapByAgent transitions the given agent's task_runs rows that are still in
// one of the supplied active statuses to the target status (ARCH-014 D3: agent
// session loss → unknown). Rows already terminal or already unknown are left
// untouched. Returns the number of rows updated.
func (m *TaskRunModel) ReapByAgent(ctx context.Context, agentID string, activeStatuses []string, targetStatus, message string) (int64, error) {
	if strings.TrimSpace(agentID) == "" {
		return 0, errors.New("agent id required")
	}
	if len(activeStatuses) == 0 {
		return 0, nil
	}
	now := time.Now()
	result := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&TaskRun{}).
		Where("agent_id = ?", agentID).
		Where("status IN ?", activeStatuses).
		Updates(map[string]interface{}{
			"status":      targetStatus,
			"message":     message,
			"finished_at": &now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ReapStale transitions every task_runs row still in one of the supplied
// active statuses whose updated_at predates `since` to the target status
// (ARCH-018 D3 watchdog: no event past the invoke budget → timed_out).
// updated_at auto-advances on every lifecycle event via GORM, so a stale
// updated_at genuinely means "no event since". Terminal, unknown, and fresh
// rows are left untouched. Returns the number of rows updated.
func (m *TaskRunModel) ReapStale(ctx context.Context, activeStatuses []string, since time.Time, targetStatus, message string) (int64, error) {
	if len(activeStatuses) == 0 {
		return 0, nil
	}
	now := time.Now()
	result := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&TaskRun{}).
		Where("status IN ?", activeStatuses).
		Where("updated_at < ?", since).
		Updates(map[string]interface{}{
			"status":      targetStatus,
			"message":     message,
			"finished_at": &now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func (m *TaskRunModel) List(ctx context.Context, opts ListTasksOptions) ([]TaskRun, int64, error) {
	opts.Normalize()

	var (
		items []TaskRun
		total int64
	)

	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&TaskRun{})
	if v := strings.TrimSpace(opts.FunctionID); v != "" {
		query = query.Where("function_id = ?", v)
	}
	if v := strings.TrimSpace(opts.Status); v != "" {
		query = query.Where("status = ?", v)
	}
	if v := strings.TrimSpace(opts.GameID); v != "" {
		query = query.Where("game_id = ?", v)
	}
	if v := strings.TrimSpace(opts.Env); v != "" {
		query = query.Where("env = ?", v)
	}
	if v := strings.TrimSpace(opts.Actor); v != "" {
		query = query.Where("actor = ?", v)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset(opts.Offset()).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// OperatorOptions 按 #23 提供 /ops/jobs「有权限的操作者」过滤下拉的
// 服务端聚合选项：同过滤维度（函数/状态/scope）下的 distinct actor 及其
// 留痕条数。空 actor（系统触发等无操作者留痕）不是可过滤选项，排除。
func (m *TaskRunModel) OperatorOptions(ctx context.Context, opts ListTasksOptions) ([]OptionCount, error) {
	opts.Normalize()

	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&TaskRun{})
	if v := strings.TrimSpace(opts.FunctionID); v != "" {
		query = query.Where("function_id = ?", v)
	}
	if v := strings.TrimSpace(opts.Status); v != "" {
		query = query.Where("status = ?", v)
	}
	if v := strings.TrimSpace(opts.GameID); v != "" {
		query = query.Where("game_id = ?", v)
	}
	if v := strings.TrimSpace(opts.Env); v != "" {
		query = query.Where("env = ?", v)
	}

	var rows []OptionCount
	if err := query.
		Where("actor <> ''").
		Select("actor AS name, COUNT(*) AS count").
		Group("actor").
		Order("count DESC, name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

type TaskEventModel struct {
	db *gorm.DB
}

func NewTaskEventModel(db *gorm.DB) *TaskEventModel {
	return &TaskEventModel{db: db}
}

func (m *TaskEventModel) Append(ctx context.Context, event *TaskEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(event).Error
}

func (m *TaskEventModel) ListByTaskID(ctx context.Context, taskID string, afterSeq int64) ([]TaskEvent, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return []TaskEvent{}, nil
	}
	var items []TaskEvent
	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Where("task_id = ?", taskID)
	if afterSeq > 0 {
		query = query.Where("seq > ?", afterSeq)
	}
	if err := query.Order("seq ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (m *TaskEventModel) NextSeq(ctx context.Context, taskID string) (int64, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 1, nil
	}
	var latest TaskEvent
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Where("task_id = ?", taskID).Order("seq DESC").First(&latest).Error
	switch {
	case err == nil:
		return latest.Seq + 1, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return 1, nil
	default:
		return 0, err
	}
}

func EncodeTaskPayload(v interface{}) JSON {
	if v == nil {
		return JSON([]byte("null"))
	}
	switch value := v.(type) {
	case []byte:
		return JSON(value)
	case string:
		return JSON([]byte(value))
	default:
		return JSON(MustJSON(v))
	}
}

func MustJSON(v interface{}) []byte {
	if v == nil {
		return []byte("null")
	}
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

// DeleteBefore 删除创建时间早于 cutoff 的任务运行记录，返回删除行数
// （R3 保留期清理，分批删除防长事务）。
func (m *TaskRunModel) DeleteBefore(ctx context.Context, cutoff time.Time, batch int) (int64, error) {
	return deleteBatch(ctx, m.db, &TaskRun{}, cutoff, batch)
}

// DeleteBefore 删除创建时间早于 cutoff 的任务事件，返回删除行数。
func (m *TaskEventModel) DeleteBefore(ctx context.Context, cutoff time.Time, batch int) (int64, error) {
	return deleteBatch(ctx, m.db, &TaskEvent{}, cutoff, batch)
}
