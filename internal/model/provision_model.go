package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// ProvisionRun 是云资源供给运行记录（OPS-001）。元数据域表（单库/meta）。
// 敏感字段（BootstrapToken）不入库——入参快照只落非敏感部分。
type ProvisionRun struct {
	ID           uint   `gorm:"primaryKey"`
	RunID        string `gorm:"size:64;uniqueIndex;not null"`
	Provider     string `gorm:"size:32;index;not null"`
	Region       string `gorm:"size:64;index"`
	GameID       string `gorm:"size:64;index"`
	Env          string `gorm:"size:32;index"`
	Policy       string `gorm:"size:16"`
	Count        int
	SpecJSON     JSON   `gorm:"type:json"`
	State        string `gorm:"size:32;index"`
	OutputsJSON  JSON   `gorm:"type:json"`
	AgentIDsJSON JSON   `gorm:"type:json"`
	ApprovalID   string `gorm:"size:64"`
	LabelsJSON   JSON   `gorm:"type:json"`
	ErrorMsg     string `gorm:"type:text"`
	RequestedBy  string `gorm:"size:64"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName 表名。
func (ProvisionRun) TableName() string { return "provision_runs" }

// ProvisionRunModel 提供供给运行 CRUD。
type ProvisionRunModel struct {
	db *gorm.DB
}

// NewProvisionRunModel 构造 CRUD 助手。
func NewProvisionRunModel(db *gorm.DB) *ProvisionRunModel {
	return &ProvisionRunModel{db: db}
}

// Create 插入运行记录。
func (m *ProvisionRunModel) Create(ctx context.Context, run *ProvisionRun) error {
	return m.db.WithContext(ctx).Create(run).Error
}

// Update 更新运行记录（全字段写回）。
func (m *ProvisionRunModel) Update(ctx context.Context, run *ProvisionRun) error {
	return m.db.WithContext(ctx).Save(run).Error
}

// FindByRunID 按 run id 查询。
func (m *ProvisionRunModel) FindByRunID(ctx context.Context, runID string) (*ProvisionRun, error) {
	var run ProvisionRun
	if err := m.db.WithContext(ctx).Where("run_id = ?", runID).First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// List 列出最近 limit 条（按 CreatedAt 倒序）。
func (m *ProvisionRunModel) List(ctx context.Context, limit int) ([]ProvisionRun, error) {
	var runs []ProvisionRun
	q := m.db.WithContext(ctx).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}
