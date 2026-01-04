// Package model provides database models for Croupier server.
package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// JobHistory records function invocation history with full context.
type JobHistory struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobID      string `gorm:"column:job_id;type:varchar(64);not null;index:idx_job_id" json:"job_id"`
	FunctionID string `gorm:"column:function_id;type:varchar(256);not null;index:idx_function_id" json:"function_id"`
	GameID     string `gorm:"column:game_id;type:varchar(64);index:idx_game_env" json:"game_id,omitempty"`
	Env        string `gorm:"column:env;type:varchar(32);index:idx_game_env" json:"env,omitempty"`

	// Actor information
	ActorID   string `gorm:"column:actor_id;type:varchar(64);index:idx_actor" json:"actor_id,omitempty"`
	ActorType string `gorm:"column:actor_type;type:varchar(32)" json:"actor_type,omitempty"` // admin, user, system

	// Execution details
	Status    string `gorm:"column:status;type:varchar(32);index:idx_status" json:"status"` // pending, running, succeeded, failed, cancelled, timeout
	AgentID   string `gorm:"column:agent_id;type:varchar(64);index:idx_agent" json:"agent_id,omitempty"`
	ServiceID string `gorm:"column:service_id;type:varchar(64)" json:"service_id,omitempty"`
	RPCAddr   string `gorm:"column:rpc_addr;type:varchar(256)" json:"rpc_addr,omitempty"`

	// Timing
	StartedAt  *time.Time `gorm:"column:started_at" json:"started_at,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;index:idx_finished_at" json:"finished_at,omitempty"`
	DurationMs int64      `gorm:"column:duration_ms" json:"duration_ms,omitempty"`

	// Data (stored as JSON)
	Payload     json.RawMessage `gorm:"column:payload;type:json" json:"payload,omitempty"`
	Result      json.RawMessage `gorm:"column:result;type:json" json:"result,omitempty"`
	ErrorMsg    string          `gorm:"column:error_msg;type:text" json:"error_msg,omitempty"`
	Metadata    json.RawMessage `gorm:"column:metadata;type:json" json:"metadata,omitempty"`
	RetryCount  int             `gorm:"column:retry_count;type:int;default:0" json:"retry_count,omitempty"`
	ParentJobID string          `gorm:"column:parent_job_id;type:varchar(64);index:idx_parent" json:"parent_job_id,omitempty"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for JobHistory.
func (JobHistory) TableName() string {
	return "job_history"
}

// JobHistoryModel provides CRUD operations for job history.
type JobHistoryModel struct {
	DB *gorm.DB
}

// NewJobHistoryModel creates a new JobHistoryModel.
func NewJobHistoryModel(db *gorm.DB) *JobHistoryModel {
	return &JobHistoryModel{DB: db}
}

// Insert creates a new job history record.
func (m *JobHistoryModel) Insert(ctx context.Context, history *JobHistory) error {
	return m.DB.WithContext(ctx).Create(history).Error
}

// FindByID retrieves a job history record by ID.
func (m *JobHistoryModel) FindByID(ctx context.Context, id int64) (*JobHistory, error) {
	var history JobHistory
	err := m.DB.WithContext(ctx).Where("id = ?", id).First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// FindByJobID retrieves job history records by job ID.
func (m *JobHistoryModel) FindByJobID(ctx context.Context, jobID string) (*JobHistory, error) {
	var history JobHistory
	err := m.DB.WithContext(ctx).Where("job_id = ?", jobID).First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// List retrieves job history records with filtering and pagination.
func (m *JobHistoryModel) List(ctx context.Context, opts *ListOptions) ([]*JobHistory, int64, error) {
	query := m.DB.WithContext(ctx).Model(&JobHistory{})

	if opts != nil {
		if opts.FunctionID != "" {
			query = query.Where("function_id = ?", opts.FunctionID)
		}
		if opts.GameID != "" {
			query = query.Where("game_id = ?", opts.GameID)
		}
		if opts.Env != "" {
			query = query.Where("env = ?", opts.Env)
		}
		if opts.Status != "" {
			query = query.Where("status = ?", opts.Status)
		}
		if opts.ActorID != "" {
			query = query.Where("actor_id = ?", opts.ActorID)
		}
		if opts.AgentID != "" {
			query = query.Where("agent_id = ?", opts.AgentID)
		}
		if !opts.StartTime.IsZero() {
			query = query.Where("created_at >= ?", opts.StartTime)
		}
		if !opts.EndTime.IsZero() {
			query = query.Where("created_at <= ?", opts.EndTime)
		}

		// Default ordering by created_at desc
		if opts.OrderBy != "" {
			query = query.Order(opts.OrderBy)
		} else {
			query = query.Order("created_at DESC")
		}

		if opts.Limit > 0 {
			query = query.Limit(opts.Limit)
		}
		if opts.Offset > 0 {
			query = query.Offset(opts.Offset)
		}
	} else {
		query = query.Order("created_at DESC").Limit(100)
	}

	var histories []*JobHistory
	var total int64

	if err := query.Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	// Count total before limit/offset
	countQuery := m.DB.WithContext(ctx).Model(&JobHistory{})
	if opts != nil {
		if opts.FunctionID != "" {
			countQuery = countQuery.Where("function_id = ?", opts.FunctionID)
		}
		if opts.GameID != "" {
			countQuery = countQuery.Where("game_id = ?", opts.GameID)
		}
		if opts.Env != "" {
			countQuery = countQuery.Where("env = ?", opts.Env)
		}
		if opts.Status != "" {
			countQuery = countQuery.Where("status = ?", opts.Status)
		}
		if opts.ActorID != "" {
			countQuery = countQuery.Where("actor_id = ?", opts.ActorID)
		}
		if opts.AgentID != "" {
			countQuery = countQuery.Where("agent_id = ?", opts.AgentID)
		}
		if !opts.StartTime.IsZero() {
			countQuery = countQuery.Where("created_at >= ?", opts.StartTime)
		}
		if !opts.EndTime.IsZero() {
			countQuery = countQuery.Where("created_at <= ?", opts.EndTime)
		}
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

// UpdateStatus updates the status and related fields of a job history record.
func (m *JobHistoryModel) UpdateStatus(ctx context.Context, jobID string, status string, finishedAt *time.Time, result json.RawMessage, errMsg string, durationMs int64) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if finishedAt != nil {
		updates["finished_at"] = finishedAt
	}
	if result != nil {
		updates["result"] = result
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	if durationMs > 0 {
		updates["duration_ms"] = durationMs
	}

	return m.DB.WithContext(ctx).
		Model(&JobHistory{}).
		Where("job_id = ?", jobID).
		Updates(updates).Error
}

// DeleteByJobID deletes a job history record by job ID.
func (m *JobHistoryModel) DeleteByJobID(ctx context.Context, jobID string) error {
	return m.DB.WithContext(ctx).Where("job_id = ?", jobID).Delete(&JobHistory{}).Error
}

// DeleteOlderThan deletes job history records older than the specified duration.
func (m *JobHistoryModel) DeleteOlderThan(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return m.DB.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&JobHistory{}).Error
}

// GetStats retrieves statistics about job history.
func (m *JobHistoryModel) GetStats(ctx context.Context, opts *ListOptions) (*JobHistoryStats, error) {
	query := m.DB.WithContext(ctx).Model(&JobHistory{})

	if opts != nil {
		if opts.FunctionID != "" {
			query = query.Where("function_id = ?", opts.FunctionID)
		}
		if opts.GameID != "" {
			query = query.Where("game_id = ?", opts.GameID)
		}
		if opts.Env != "" {
			query = query.Where("env = ?", opts.Env)
		}
		if opts.ActorID != "" {
			query = query.Where("actor_id = ?", opts.ActorID)
		}
		if !opts.StartTime.IsZero() {
			query = query.Where("created_at >= ?", opts.StartTime)
		}
		if !opts.EndTime.IsZero() {
			query = query.Where("created_at <= ?", opts.EndTime)
		}
	}

	var stats JobHistoryStats

	// Count by status
	rows, err := query.Select("status, count(*) as count").
		Group("status").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case "succeeded":
			stats.Succeeded = count
		case "failed":
			stats.Failed = count
		case "running":
			stats.Running = count
		case "cancelled":
			stats.Cancelled = count
		case "timeout":
			stats.Timeout = count
		default:
			stats.Other += count
		}
		stats.Total += count
	}

	// Get average duration for succeeded jobs
	var avgDuration sql.NullFloat64
	if err := query.Where("status = ?", "succeeded").
		Select("AVG(duration_ms)").
		Scan(&avgDuration).Error; err == nil && avgDuration.Valid {
		stats.AvgDurationMs = int64(avgDuration.Float64)
	}

	return &stats, nil
}

// ListOptions provides filtering and pagination options for listing job history.
type ListOptions struct {
	FunctionID string
	GameID     string
	Env        string
	Status     string
	ActorID    string
	AgentID    string
	StartTime  time.Time
	EndTime    time.Time
	OrderBy    string
	Limit      int
	Offset     int
}

// JobHistoryStats provides statistics about job history.
type JobHistoryStats struct {
	Total         int64
	Succeeded     int64
	Failed        int64
	Running       int64
	Cancelled     int64
	Timeout       int64
	Other         int64
	AvgDurationMs int64
}
