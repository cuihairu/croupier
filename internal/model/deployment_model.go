package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// DeployPackage 是运维部署包仓库条目（OPS-004）。元数据域表（单库/meta）。
// 构建产物存于对象存储（ObjectKey），大小/校验和在上传时由服务端计算。
type DeployPackage struct {
	ID          uint   `gorm:"primaryKey"`
	PackageID   string `gorm:"size:64;uniqueIndex;not null"`
	Name        string `gorm:"size:128;index"`
	Version     string `gorm:"size:64"`
	GameID      string `gorm:"size:64;index"`
	Env         string `gorm:"size:64;index"`
	Role        string `gorm:"size:64;index"` // 目标角色（game-server|agent|...）
	ObjectKey   string `gorm:"size:512"`
	Size        int64
	Checksum    string `gorm:"size:64"` // sha256 hex
	ContentType string `gorm:"size:128"`
	CreatedBy   string `gorm:"size:64"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName 表名。
func (DeployPackage) TableName() string { return "deploy_packages" }

// Deployment 是运维发版单（OPS-004）。
type Deployment struct {
	ID            uint   `gorm:"primaryKey"`
	DeploymentID  string `gorm:"size:64;uniqueIndex;not null"`
	PackageID     string `gorm:"size:64;index;not null"`
	Name          string `gorm:"size:128"`
	GameID        string `gorm:"size:64;index"`
	Env           string `gorm:"size:64;index"`
	Role          string `gorm:"size:64;index"`
	ProcessName   string `gorm:"size:128"` // agent 侧受管进程名
	BatchPercent  int    `gorm:"default:100"`
	BatchCount    int    `gorm:"default:1"`
	State         string `gorm:"size:32;index"`
	TargetIDsJSON JSON   `gorm:"type:json"` // []string 目标 agent id
	TargetsJSON   JSON   `gorm:"type:json"` // []DeploymentTarget 分批快照
	PausedReason  string `gorm:"type:text"`
	CreatedBy     string `gorm:"size:64"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName 表名。
func (Deployment) TableName() string { return "deployments" }

// DeploymentTarget 是发版单内单个目标机的执行状态（存于 TargetIDsJSON/
// TargetsJSON）。
type DeploymentTarget struct {
	AgentID        string `json:"agentId"`
	Batch          int    `json:"batch"`
	State          string `json:"state"` // pending|deploying|healthy|failed|skipped|rolled_back
	Message        string `json:"message,omitempty"`
	OldFingerprint string `json:"oldFingerprint,omitempty"`
	NewFingerprint string `json:"newFingerprint,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

// Deployment 状态机。
const (
	DeploymentStateDraft          = "draft"
	DeploymentStateDeploying      = "deploying"
	DeploymentStatePaused         = "paused"
	DeploymentStateComplete       = "complete"
	DeploymentStateFailed         = "failed"
	DeploymentStateRolledBack     = "rolled_back"
	DeploymentStateRollbackFailed = "rollback_failed"
)

// DeploymentTarget 状态机。
const (
	DeploymentTargetPending    = "pending"
	DeploymentTargetDeploying  = "deploying"
	DeploymentTargetHealthy    = "healthy"
	DeploymentTargetFailed     = "failed"
	DeploymentTargetSkipped    = "skipped"
	DeploymentTargetRolledBack = "rolled_back"
)

// DeploymentModel 提供发版单 CRUD。
type DeploymentModel struct {
	db *gorm.DB
}

// NewDeploymentModel 构造。
func NewDeploymentModel(db *gorm.DB) *DeploymentModel {
	return &DeploymentModel{db: db}
}

// Create 插入。
func (m *DeploymentModel) Create(ctx context.Context, d *Deployment) error {
	return m.db.WithContext(ctx).Create(d).Error
}

// Update 全字段写回。
func (m *DeploymentModel) Update(ctx context.Context, d *Deployment) error {
	return m.db.WithContext(ctx).Save(d).Error
}

// CompareAndSwapState 仅当当前 state == from 时原子更新为 to（含整行写回）。
// 返回 (成功, 当前行)。用于 Start 的 draft→deploying 非原子读改写并发防护。
func (m *DeploymentModel) CompareAndSwapState(ctx context.Context, id, from, to string) (bool, *Deployment, error) {
	var d Deployment
	err := m.db.WithContext(ctx).Where("deployment_id = ? AND state = ?", id, from).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	d.State = to
	d.UpdatedAt = time.Now()
	res := m.db.WithContext(ctx).Model(&d).Where("deployment_id = ? AND state = ?", id, from).
		Updates(map[string]any{"state": to, "updated_at": d.UpdatedAt})
	if res.Error != nil {
		return false, nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 并发 Start 已把 state 改走：CAS 失败，无整行可回读。
		return false, nil, nil
	}
	// 回读整行（含 targets 等）供编排继续。
	row, err := m.FindByID(ctx, id)
	if err != nil {
		return false, nil, err
	}
	return true, row, nil
}

// FindByID 按 deployment id 查询。
func (m *DeploymentModel) FindByID(ctx context.Context, id string) (*Deployment, error) {
	var d Deployment
	if err := m.db.WithContext(ctx).Where("deployment_id = ?", id).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// List 列出最近 limit 条。
func (m *DeploymentModel) List(ctx context.Context, limit int) ([]Deployment, error) {
	var ds []Deployment
	q := m.db.WithContext(ctx).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&ds).Error; err != nil {
		return nil, err
	}
	return ds, nil
}

// DeployPackageModel 提供包仓库 CRUD。
type DeployPackageModel struct {
	db *gorm.DB
}

// NewDeployPackageModel 构造。
func NewDeployPackageModel(db *gorm.DB) *DeployPackageModel {
	return &DeployPackageModel{db: db}
}

// Create 插入。
func (m *DeployPackageModel) Create(ctx context.Context, p *DeployPackage) error {
	return m.db.WithContext(ctx).Create(p).Error
}

// FindByID 按 package id 查询。
func (m *DeployPackageModel) FindByID(ctx context.Context, id string) (*DeployPackage, error) {
	var p DeployPackage
	if err := m.db.WithContext(ctx).Where("package_id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// List 列出最近 limit 条。
func (m *DeployPackageModel) List(ctx context.Context, limit int) ([]DeployPackage, error) {
	var ps []DeployPackage
	q := m.db.WithContext(ctx).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
}
