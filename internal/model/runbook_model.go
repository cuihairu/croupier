package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Runbook step kinds (OPS-009): each step references an OPS-004 deploy
// capability (start/rollback) or an OPS-006 batch capability (config push /
// restricted exec).
const (
	RunbookStepBatchExec      = "batch_exec"
	RunbookStepBatchPush      = "batch_push"
	RunbookStepDeployStart    = "deploy_start"
	RunbookStepDeployRollback = "deploy_rollback"
)

// RunbookRunStatus is the status of one runbook execution.
const (
	RunbookRunStatusRunning   = "running"
	RunbookRunStatusSucceeded = "succeeded"
	RunbookRunStatusPartial   = "partial"
	RunbookRunStatusFailed    = "failed"
)

// Runbook is one OPS-009 runbook: a parameterized ordered step sequence that
// references OPS-004 deploy / OPS-006 batch capabilities. StepsJSON carries
// the array of StepSpec objects.
type Runbook struct {
	ID uint `gorm:"primaryKey"`
	// RunbookID is a stable public id (time-prefixed random).
	RunbookID   string `gorm:"size:64;uniqueIndex;not null"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"size:512"`
	GameID      string `gorm:"size:64;index:idx_runbook_scope;not null"`
	Env         string `gorm:"size:64;index:idx_runbook_scope;not null"`
	StepsJSON   string `gorm:"type:text;not null"`
	CreatedBy   string `gorm:"size:64"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Runbook) TableName() string { return "runbooks" }

// RunbookRun is one execution of a Runbook. ParamsJSON carries the parameter
// substitutions; StepsResultJSON carries the per-step outcomes.
type RunbookRun struct {
	ID              uint   `gorm:"primaryKey"`
	RunID           string `gorm:"size:64;uniqueIndex;not null"`
	RunbookID       string `gorm:"size:64;index:idx_runbook_run_scope;not null"`
	GameID          string `gorm:"size:64;index:idx_runbook_run_scope;not null"`
	Env             string `gorm:"size:64;index:idx_runbook_run_scope;not null"`
	ParamsJSON      string `gorm:"type:text"`
	Status          string `gorm:"size:16;not null"`
	CurrentStep     int    `gorm:"not null;default:0"`
	StepsResultJSON string `gorm:"type:text"`
	CreatedBy       string `gorm:"size:64"`
	StartedAt       time.Time
	FinishedAt      *time.Time
	Error           string `gorm:"type:text"`
}

func (RunbookRun) TableName() string { return "runbook_runs" }

// RunbookModel persists runbooks (scoped by game/env).
type RunbookModel struct{ db *gorm.DB }

func NewRunbookModel(db *gorm.DB) *RunbookModel { return &RunbookModel{db: db} }

func (m *RunbookModel) Create(ctx context.Context, rb *Runbook) error {
	return m.db.WithContext(ctx).Create(rb).Error
}

func (m *RunbookModel) Find(ctx context.Context, gameID, env, runbookID string) (*Runbook, error) {
	var rb Runbook
	err := m.db.WithContext(ctx).Where("game_id=? AND env=? AND runbook_id=?", gameID, env, runbookID).Take(&rb).Error
	if err != nil {
		return nil, err
	}
	return &rb, nil
}

func (m *RunbookModel) List(ctx context.Context, gameID, env string, limit int) ([]Runbook, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []Runbook
	err := m.db.WithContext(ctx).Where("game_id=? AND env=?", gameID, env).Order("created_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (m *RunbookModel) Delete(ctx context.Context, gameID, env, runbookID string) error {
	res := m.db.WithContext(ctx).Where("game_id=? AND env=? AND runbook_id=?", gameID, env, runbookID).Delete(&Runbook{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RunbookRunModel persists runbook executions.
type RunbookRunModel struct{ db *gorm.DB }

func NewRunbookRunModel(db *gorm.DB) *RunbookRunModel { return &RunbookRunModel{db: db} }

func (m *RunbookRunModel) Create(ctx context.Context, run *RunbookRun) error {
	return m.db.WithContext(ctx).Create(run).Error
}

func (m *RunbookRunModel) Find(ctx context.Context, gameID, env, runID string) (*RunbookRun, error) {
	var run RunbookRun
	err := m.db.WithContext(ctx).Where("game_id=? AND env=? AND run_id=?", gameID, env, runID).Take(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (m *RunbookRunModel) ListByRunbook(ctx context.Context, gameID, env, runbookID string, limit int) ([]RunbookRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []RunbookRun
	err := m.db.WithContext(ctx).Where("game_id=? AND env=? AND runbook_id=?", gameID, env, runbookID).
		Order("started_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

// ListRunning 返回 started_at 早于 olderThan 且仍为 running 的运行行。启动恢复
// 用：进程重启前遗留的终态持久化失败行。限定 olderThan 避免多实例下把另一实例
// 仍在执行的运行误判为遗留。
func (m *RunbookRunModel) ListRunning(ctx context.Context, olderThan time.Time) ([]RunbookRun, error) {
	var out []RunbookRun
	err := m.db.WithContext(ctx).
		Where("status = ? AND started_at < ?", RunbookRunStatusRunning, olderThan).
		Order("started_at ASC").
		Find(&out).Error
	return out, err
}

// UpdateStatus 终态写回：status/currentStep/stepsResult/error/finishedAt。
func (m *RunbookRunModel) UpdateStatus(ctx context.Context, id uint, status string, currentStep int, stepsResultJSON, errMsg string) error {
	if id == 0 {
		return errors.New("runbook run id required")
	}
	now := time.Now().UTC()
	return m.db.WithContext(ctx).Model(&RunbookRun{}).Where("id=?", id).Updates(map[string]any{
		"status":            status,
		"current_step":      currentStep,
		"steps_result_json": stepsResultJSON,
		"error":             errMsg,
		"finished_at":       now,
	}).Error
}
