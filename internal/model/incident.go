package model

import (
	"context"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Incident 事故登记（docs/design/incident-reports.md §2.2）：人工登记 /
// 外部 API / 自动源一键转换三入口，按 category_id 关联配置表类别（职能域）。
// meta DB 存储（跨 game 聚合报表是主用例，game_id 仅作过滤列）。
type Incident struct {
	gorm.Model
	IncidentKey *string `gorm:"size:128;uniqueIndex:uidx_incidents_key"` // 幂等键；人工登记为 NULL（唯一索引允许多 NULL），外部/自动源必填
	Title       string  `gorm:"size:255;not null"`
	CategoryID  uint    `gorm:"index"`
	Subcategory string  `gorm:"size:64"`
	Severity    string  `gorm:"size:16;index"`
	Status      string  `gorm:"size:16;index"`
	Source      string  `gorm:"size:32;index"` // manual|external|alert|cicd|probe|supervisor|bug
	// 责任归因（登记时快照；execlog 保留期有限，归因不事后反查）
	ResponsibleType string `gorm:"size:16;index"` // agent|operator|change|unknown
	ResponsibleID   string `gorm:"size:255;index"`
	// 时间线（时长口径基准：影响时长/MTTR）
	DetectedAt time.Time `gorm:"index"`
	ResolvedAt *time.Time
	// 关联
	GameID     string            `gorm:"size:64;index"`
	Env        string            `gorm:"size:64;index"`
	ExecLogIDs JSON              `gorm:"type:json"` // 关联 execution_logs.ID 列表（溯源引用）
	RefType    string            `gorm:"size:32"`   // alert|cicd_build|bug|probe_window|supervisor_event
	RefID      string            `gorm:"size:128;index"`
	Details    datatypes.JSONMap `gorm:"type:json"`
	CreatedBy  string            `gorm:"size:64"` // manual=操作者；external=token 名
}

func (Incident) TableName() string { return "incidents" }

// 事故状态机：open → acknowledged → resolved；resolved 可重开（清 ResolvedAt）。
const (
	IncidentStatusOpen         = "open"
	IncidentStatusAcked        = "acknowledged"
	IncidentStatusResolved     = "resolved"
	IncidentSeverityInfo       = "info"
	IncidentSeverityWarning    = "warning"
	IncidentSeverityCritical   = "critical"
	IncidentSourceManual       = "manual"
	IncidentSourceExternal     = "external"
	IncidentSourceAlert        = "alert"
	IncidentSourceCicd         = "cicd"
	IncidentSourceProbe        = "probe"
	IncidentSourceSupervisor   = "supervisor"
	IncidentSourceBug          = "bug"
	IncidentRespAgent          = "agent"
	IncidentRespOperator       = "operator"
	IncidentRespChange         = "change"
	IncidentRespUnknown        = "unknown"
	IncidentRefAlert           = "alert"
	IncidentRefCicdBuild       = "cicd_build"
	IncidentRefBug             = "bug"
	IncidentRefProbeWindow     = "probe_window"
	IncidentRefSupervisorEvent = "supervisor_event"
)

// ValidIncidentStatuses / ValidIncidentSeverities / ValidIncidentSources /
// ValidIncidentRespTypes / ValidIncidentRefTypes 是登记与流转的闭集校验表。
var (
	ValidIncidentStatuses = map[string]struct{}{
		IncidentStatusOpen: {}, IncidentStatusAcked: {}, IncidentStatusResolved: {},
	}
	ValidIncidentSeverities = map[string]struct{}{
		IncidentSeverityInfo: {}, IncidentSeverityWarning: {}, IncidentSeverityCritical: {},
	}
	ValidIncidentSources = map[string]struct{}{
		IncidentSourceManual: {}, IncidentSourceExternal: {}, IncidentSourceAlert: {},
		IncidentSourceCicd: {}, IncidentSourceProbe: {}, IncidentSourceSupervisor: {},
		IncidentSourceBug: {},
	}
	ValidIncidentRespTypes = map[string]struct{}{
		IncidentRespAgent: {}, IncidentRespOperator: {}, IncidentRespChange: {}, IncidentRespUnknown: {},
	}
	ValidIncidentRefTypes = map[string]struct{}{
		IncidentRefAlert: {}, IncidentRefCicdBuild: {}, IncidentRefBug: {},
		IncidentRefProbeWindow: {}, IncidentRefSupervisorEvent: {},
	}
)

// IncidentQueryOptions 控制事故列表过滤。
type IncidentQueryOptions struct {
	PaginationOptions
	CategoryID      uint
	Subcategory     string
	Status          string
	Severity        string
	Source          string
	ResponsibleType string
	ResponsibleID   string
	GameID          string
	Env             string
	From            *time.Time
	To              *time.Time
}

// IncidentModel 提供 incidents 的数据访问。
type IncidentModel struct {
	db *gorm.DB
}

func NewIncidentModel(db *gorm.DB) *IncidentModel {
	return &IncidentModel{db: db}
}

func (m *IncidentModel) Create(ctx context.Context, row *Incident) error {
	return m.db.WithContext(ctx).Create(row).Error
}

func (m *IncidentModel) Get(ctx context.Context, id uint) (*Incident, error) {
	var row Incident
	if err := m.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// FindByKey 按幂等键取事故（外部 API 幂等重放判定）。
func (m *IncidentModel) FindByKey(ctx context.Context, key string) (*Incident, error) {
	var row Incident
	err := m.db.WithContext(ctx).Where("incident_key = ?", key).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (m *IncidentModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Incident{}).Where("id = ?", id).Updates(updates).Error
}

// List 按过滤条件分页查询（detected_at 倒序）。
func (m *IncidentModel) List(ctx context.Context, opts IncidentQueryOptions) ([]Incident, int64, error) {
	opts.Normalize()
	query := m.db.WithContext(ctx).Model(&Incident{})
	if opts.CategoryID != 0 {
		query = query.Where("category_id = ?", opts.CategoryID)
	}
	if opts.Subcategory != "" {
		query = query.Where("subcategory = ?", opts.Subcategory)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Severity != "" {
		query = query.Where("severity = ?", opts.Severity)
	}
	if opts.Source != "" {
		query = query.Where("source = ?", opts.Source)
	}
	if opts.ResponsibleType != "" {
		query = query.Where("responsible_type = ?", opts.ResponsibleType)
	}
	if opts.ResponsibleID != "" {
		query = query.Where("responsible_id = ?", opts.ResponsibleID)
	}
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.From != nil {
		query = query.Where("detected_at >= ?", *opts.From)
	}
	if opts.To != nil {
		query = query.Where("detected_at < ?", *opts.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Incident
	err := query.Order("detected_at DESC, id DESC").
		Offset(opts.Offset()).Limit(opts.PageSize).Find(&rows).Error
	return rows, total, err
}

// ReassignCategory 把某类别下全部事故归并到目标类别（删除类别级联）。
func (m *IncidentModel) ReassignCategory(ctx context.Context, fromID, toID uint) error {
	return m.db.WithContext(ctx).Model(&Incident{}).
		Where("category_id = ?", fromID).
		Update("category_id", toID).Error
}

// CountByCategory 某类别下的事故数（删除级联提示）。
func (m *IncidentModel) CountByCategory(ctx context.Context, categoryID uint) (int64, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&Incident{}).
		Where("category_id = ?", categoryID).Count(&count).Error
	return count, err
}
