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
	// DestroyApprovalID 是 workflow 模式销毁申请单的审批记录 id（OPS-003
	// 迁移 0054 补列；存量行空=未申请销毁审批，行为与现状一致）。
	DestroyApprovalID string `gorm:"size:64"`
	LabelsJSON        JSON   `gorm:"type:json"`
	// CloudInitExtra 是追加到 cloud-init 的自定义片段（迁移 0055 补列；
	// 存量行空=未提供，行为与现状一致）。非敏感，可下库。
	CloudInitExtra string `gorm:"type:text"`
	ErrorMsg       string `gorm:"type:text"`
	RequestedBy    string `gorm:"size:64"`
	// DestroyProtected 是数据盘护栏（OPS-002）：true 时拒绝自动销毁（迁移
	// 0053 补列，存量行默认 false=未护栏，行为与现状一致）。
	DestroyProtected bool `gorm:"default:false"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
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

// ListAdoptedScope 列出指定 scope 的 adopted 运行（按 CreatedAt 倒序）。
// scope 条件必须下推 SQL，避免先全表/全局 limit 再内存过滤导致较早目标记录被漏计。
func (m *ProvisionRunModel) ListAdoptedScope(ctx context.Context, gameID, env string, limit int) ([]ProvisionRun, error) {
	var runs []ProvisionRun
	q := m.db.WithContext(ctx).
		Where("state = ?", "adopted").
		Order("created_at DESC")
	if gameID != "" {
		q = q.Where("game_id = ?", gameID)
	}
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}
