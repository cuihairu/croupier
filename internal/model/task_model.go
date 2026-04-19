package model

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/datatypes"
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
	return m.db.WithContext(ctx).Create(task).Error
}

func (m *TaskRunModel) FindByTaskID(ctx context.Context, taskID string) (*TaskRun, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var task TaskRun
	if err := m.db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (m *TaskRunModel) UpdateByTaskID(ctx context.Context, taskID string, updates map[string]interface{}) error {
	if strings.TrimSpace(taskID) == "" {
		return errors.New("task id required")
	}
	return m.db.WithContext(ctx).Model(&TaskRun{}).Where("task_id = ?", taskID).Updates(updates).Error
}

func (m *TaskRunModel) List(ctx context.Context, opts ListTasksOptions) ([]TaskRun, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []TaskRun
		total int64
	)

	query := m.db.WithContext(ctx).Model(&TaskRun{})
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
	return m.db.WithContext(ctx).Create(event).Error
}

func (m *TaskEventModel) ListByTaskID(ctx context.Context, taskID string, afterSeq int64) ([]TaskEvent, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return []TaskEvent{}, nil
	}
	var items []TaskEvent
	query := m.db.WithContext(ctx).Where("task_id = ?", taskID)
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
	err := m.db.WithContext(ctx).Where("task_id = ?", taskID).Order("seq DESC").First(&latest).Error
	switch {
	case err == nil:
		return latest.Seq + 1, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return 1, nil
	default:
		return 0, err
	}
}

func EncodeTaskPayload(v interface{}) datatypes.JSON {
	if v == nil {
		return datatypes.JSON([]byte("null"))
	}
	switch value := v.(type) {
	case []byte:
		return datatypes.JSON(value)
	case string:
		return datatypes.JSON([]byte(value))
	default:
		return datatypes.JSON(MustJSON(v))
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
