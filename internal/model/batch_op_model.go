package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// BatchOpStatus 批操作状态（OPS-006）。
const (
	BatchOpStatusDraft     = "draft"
	BatchOpStatusDryRun    = "dry_run"
	BatchOpStatusRunning   = "running"
	BatchOpStatusCompleted = "completed"
	BatchOpStatusFailed    = "failed"
)

// BatchOpKind 批操作类型。
const (
	BatchOpKindConfig = "config"
	BatchOpKindExec   = "exec"
)

// BatchOp is one OPS-006 batch operation: config push or restricted command
// execution against a selected agent set. TargetsJSON carries the resolved
// agent list; ResultsJSON carries per-agent outcomes for the aggregate report.
type BatchOp struct {
	ID uint `gorm:"primaryKey"`
	// BatchID is a stable public id (ULID-ish).
	BatchID string `gorm:"size:64;uniqueIndex;not null"`
	GameID  string `gorm:"size:64;index:idx_batch_op_scope;not null"`
	Env     string `gorm:"size:64;index:idx_batch_op_scope;not null"`
	Kind    string `gorm:"size:16;not null"`
	// LabelsJSON 圈选标签（AND 精确匹配）。
	LabelsJSON string `gorm:"type:text"`
	// AgentsJSON 显式 agent id 列表（与标签同时给时取交集）。
	AgentsJSON string `gorm:"type:text"`
	// PayloadJSON 配置下发（path/content base64/mode/force/sha256）或命令
	//（command/args/timeoutSeconds）。Dry-run 结果不落敏感明文，仅摘要。
	PayloadJSON string `gorm:"type:text;not null"`
	DryRun      bool
	Status      string `gorm:"size:16;index;not null"`
	Targets     int    `gorm:"not null;default:0"`
	Succeeded   int    `gorm:"not null;default:0"`
	Failed      int    `gorm:"not null;default:0"`
	Skipped     int    `gorm:"not null;default:0"`
	CreatedBy   string `gorm:"size:64"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ResultsJSON string `gorm:"type:text"`
}

func (BatchOp) TableName() string { return "batch_ops" }

type BatchOpModel struct{ db *gorm.DB }

func NewBatchOpModel(db *gorm.DB) *BatchOpModel { return &BatchOpModel{db: db} }

func (m *BatchOpModel) Create(ctx context.Context, op *BatchOp) error {
	return m.db.WithContext(ctx).Create(op).Error
}

func (m *BatchOpModel) Find(ctx context.Context, gameID, env, batchID string) (*BatchOp, error) {
	var op BatchOp
	err := m.db.WithContext(ctx).Where("game_id=? AND env=? AND batch_id=?", gameID, env, batchID).Take(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (m *BatchOpModel) List(ctx context.Context, gameID, env string, limit int) ([]BatchOp, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var out []BatchOp
	err := m.db.WithContext(ctx).Where("game_id=? AND env=?", gameID, env).Order("created_at DESC").Limit(limit).Find(&out).Error
	return out, err
}

func (m *BatchOpModel) UpdateStatus(ctx context.Context, id uint, status string, fields map[string]any) error {
	if id == 0 {
		return errors.New("batch op id required")
	}
	tx := m.db.WithContext(ctx).Model(&BatchOp{}).Where("id=?", id)
	if len(fields) > 0 {
		updates := make(map[string]any, len(fields)+1)
		for k, v := range fields {
			updates[k] = v
		}
		updates["status"] = status
		updates["updated_at"] = time.Now().UTC()
		return tx.Updates(updates).Error
	}
	return tx.Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error
}

// CompareAndSwapStatus 仅当当前 status == from 时原子更新为 to（含回读行）。
// 返回 (成功, 当前行)；ok=false 表示并发已把状态改走（#664 状态闸）。
func (m *BatchOpModel) CompareAndSwapStatus(ctx context.Context, id uint, from, to string) (bool, *BatchOp, error) {
	res := m.db.WithContext(ctx).Model(&BatchOp{}).Where("id=? AND status=?", id, from).
		Updates(map[string]any{"status": to, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return false, nil, res.Error
	}
	if res.RowsAffected == 0 {
		var cur BatchOp
		if err := m.db.WithContext(ctx).Where("id=?", id).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil, nil
			}
			return false, nil, err
		}
		return false, &cur, nil
	}
	var cur BatchOp
	if err := m.db.WithContext(ctx).Where("id=?", id).First(&cur).Error; err != nil {
		return false, nil, err
	}
	return true, &cur, nil
}
