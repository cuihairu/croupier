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

type ListTasksOptions struct {
	PaginationOptions
	FunctionID string
	Status     string
	GameID     string
	Env        string
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

func (m *TaskRunModel) List(ctx context.Context, opts ListTasksOptions) ([]TaskRun, int64, error) {
	opts.PaginationOptions.Normalize()

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

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset(opts.Offset()).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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
