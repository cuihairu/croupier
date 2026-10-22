package model

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
)

// IncidentCategory 是事故报表体系的事故类别（职能域），全部存库可配：
// 名称/leader/子类白名单/可见范围/启停都可改，slug 建行后不可改（外部 API
// 与报表 payload 按 slug 稳定引用）。设计见 docs/design/incident-reports.md §2.1/§3。
type IncidentCategory struct {
	gorm.Model
	Name          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_name"`
	Slug          string `gorm:"size:64;not null;uniqueIndex:uidx_incident_categories_slug"`
	Sort          int
	Leader        string `gorm:"size:64;index"`
	Subcategories JSON   `gorm:"type:json"` // 子类白名单（字符串数组，空=不限子类）
	Audience      JSON   `gorm:"type:json"` // 可见范围 {roles:[], users:[]}
	Enabled       bool
	Builtin       bool // 未分类与播种行标记；未分类不可删、slug 不可改
}

func (IncidentCategory) TableName() string { return "incident_categories" }

// 类别 slug 稳定标识（播种值；名称/leader/子类可改，slug 不可改）。
const (
	IncidentCatUncategorized = "uncategorized"
	IncidentCatFrontend      = "frontend"
	IncidentCatClient        = "client"
	IncidentCatArt           = "art"
	IncidentCatDesign        = "design"
	IncidentCatQA            = "qa"
	IncidentCatOps           = "ops"
)

// incidentCategorySeed 是一条默认类别的播种定义。
type incidentCategorySeed struct {
	Slug          string
	Name          string
	Sort          int
	Builtin       bool
	Subcategories []string
}

// incidentCategorySeeds 是 0040 迁移与首启共用的默认类别（未分类兜底 + 六类职能域）。
// 子类白名单是初始样例，后续全在设置页改。
func incidentCategorySeeds() []incidentCategorySeed {
	return []incidentCategorySeed{
		{Slug: IncidentCatUncategorized, Name: "未分类", Sort: 0, Builtin: true},
		{Slug: IncidentCatFrontend, Name: "前端", Sort: 1, Builtin: true},
		{Slug: IncidentCatClient, Name: "客户端", Sort: 2, Builtin: true,
			Subcategories: []string{"崩溃", "性能", "网络"}},
		{Slug: IncidentCatArt, Name: "美术", Sort: 3, Builtin: true},
		{Slug: IncidentCatDesign, Name: "策划", Sort: 4, Builtin: true},
		{Slug: IncidentCatQA, Name: "测试", Sort: 5, Builtin: true},
		{Slug: IncidentCatOps, Name: "运维", Sort: 6, Builtin: true,
			Subcategories: []string{"部署", "配置", "探活", "编译"}},
	}
}

// SeedIncidentCategories 幂等播种：按 slug 缺行才插，已有行不回改。
func SeedIncidentCategories(ctx context.Context, db *gorm.DB) error {
	m := NewIncidentCategoryModel(db)
	for _, seed := range incidentCategorySeeds() {
		var count int64
		if err := m.db.WithContext(ctx).Model(&IncidentCategory{}).
			Where("slug = ?", seed.Slug).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		row := &IncidentCategory{
			Name:    seed.Name,
			Slug:    seed.Slug,
			Sort:    seed.Sort,
			Enabled: true,
			Builtin: seed.Builtin,
		}
		if len(seed.Subcategories) > 0 {
			// []string 经 json.Marshal 恒成功
			subRaw, _ := json.Marshal(seed.Subcategories)
			row.Subcategories = JSON(subRaw)
		}
		if err := m.Create(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

// IncidentCategoryModel 提供 incident_categories 的数据访问。
type IncidentCategoryModel struct {
	db *gorm.DB
}

func NewIncidentCategoryModel(db *gorm.DB) *IncidentCategoryModel {
	return &IncidentCategoryModel{db: db}
}

func (m *IncidentCategoryModel) Create(ctx context.Context, row *IncidentCategory) error {
	return m.db.WithContext(ctx).Create(row).Error
}

func (m *IncidentCategoryModel) Get(ctx context.Context, id uint) (*IncidentCategory, error) {
	var row IncidentCategory
	if err := m.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetBySlug 按 slug 取类别（外部 API 用 slug 引用）。
func (m *IncidentCategoryModel) GetBySlug(ctx context.Context, slug string) (*IncidentCategory, error) {
	var row IncidentCategory
	if err := m.db.WithContext(ctx).Where("slug = ?", slug).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// List 全量类别（含停用；enabledOnly=true 只取启用行，登记选择器用）。
func (m *IncidentCategoryModel) List(ctx context.Context, enabledOnly bool) ([]IncidentCategory, error) {
	query := m.db.WithContext(ctx).Model(&IncidentCategory{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var rows []IncidentCategory
	err := query.Order("sort ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (m *IncidentCategoryModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	res := m.db.WithContext(ctx).Model(&IncidentCategory{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		if err := m.db.WithContext(ctx).First(&IncidentCategory{}, id).Error; err != nil {
			return err
		}
	}
	return nil
}

// Delete 物理删行；builtin 保护由调用方（service 层）裁决。
func (m *IncidentCategoryModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&IncidentCategory{}, id).Error
}

// CountEnabledByIDs 校验一组类别 ID 是否全部为启用行（登记校验用，返回缺失 ID）。
func (m *IncidentCategoryModel) MissingEnabledIDs(ctx context.Context, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []IncidentCategory
	if err := m.db.WithContext(ctx).
		Where("id IN ? AND enabled = ?", ids, true).Find(&rows).Error; err != nil {
		return nil, err
	}
	seen := make(map[uint]bool, len(rows))
	for _, r := range rows {
		seen[r.ID] = true
	}
	var missing []uint
	for _, id := range ids {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// UncategorizedID 取未分类兜底行 ID（删除类别归并目标；缺失视为数据异常报错）。
func (m *IncidentCategoryModel) UncategorizedID(ctx context.Context) (uint, error) {
	var row IncidentCategory
	err := m.db.WithContext(ctx).Where("slug = ?", IncidentCatUncategorized).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, errors.New("incident category: uncategorized row missing")
	}
	return row.ID, err
}
