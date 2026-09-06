package model

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ComponentTemplate 是可复用的页面组件模板（V4 核心实体）：
// 多个函数 + 布局 + 联动封装为一个可拖入画布的组件。
// 内置模板（Builtin=true）由契约扫描自动生成；用户可保存自定义模板。
type ComponentTemplate struct {
	gorm.Model
	// Key 唯一标识（如 player-management）
	Key string `gorm:"size:128;uniqueIndex;not null"`
	// Name 显示名（JSON LocalizedText）
	Name JSON `gorm:"type:json;not null"`
	// Description 描述（JSON LocalizedText）
	Description JSON `gorm:"type:json"`
	// Category 分类：运营 / 客服 / 数据 / 配置 / 自定义
	Category string `gorm:"size:32;index"`
	// Icon antd icon 名
	Icon string `gorm:"size:64"`
	// RequiredFunctions 依赖的函数 ID 列表（拖入时检查 scope 可用性）
	RequiredFunctions JSON `gorm:"type:json"`
	// Params 参数定义列表（U6 模板参数化）：每项 {key,label,nodeId,prop,default}，
	// 实例化时按参数值替换对应节点的白名单 prop（title/span/autoRun）。
	Params JSON `gorm:"type:json"`
	// Tree 页面组件子树（与编辑器 PageNode 同构的 JSON 序列化）
	Tree JSON `gorm:"type:json;not null"`
	// Builtin 是否为内置模板（契约自动生成 vs 用户保存）
	Builtin bool `gorm:"default:false;index"`
	// CreatedBy 创建者
	CreatedBy string `gorm:"size:64"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ComponentTemplateModel is the DB access layer.
type ComponentTemplateModel struct {
	db *gorm.DB
}

// NewComponentTemplateModel creates a model instance.
func NewComponentTemplateModel(db *gorm.DB) *ComponentTemplateModel {
	return &ComponentTemplateModel{db: db}
}

// Create inserts a new template.
func (m *ComponentTemplateModel) Create(ctx context.Context, t *ComponentTemplate) error {
	t.Key = strings.TrimSpace(t.Key)
	if t.Key == "" {
		return ErrComponentTemplateKeyRequired
	}
	return m.db.WithContext(ctx).Create(t).Error
}

// UpsertBuiltin inserts or updates a builtin template by key.
func (m *ComponentTemplateModel) UpsertBuiltin(ctx context.Context, t *ComponentTemplate) error {
	t.Key = strings.TrimSpace(t.Key)
	if t.Key == "" {
		return ErrComponentTemplateKeyRequired
	}
	t.Builtin = true
	var existing ComponentTemplate
	err := m.db.WithContext(ctx).Where("key = ?", t.Key).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return m.db.WithContext(ctx).Create(t).Error
	}
	if err != nil {
		return err
	}
	existing.Name = t.Name
	existing.Description = t.Description
	existing.Category = t.Category
	existing.Icon = t.Icon
	existing.RequiredFunctions = t.RequiredFunctions
	existing.Tree = t.Tree
	return m.db.WithContext(ctx).Save(&existing).Error
}

// FindByKey returns a template by its key.
func (m *ComponentTemplateModel) FindByKey(ctx context.Context, key string) (*ComponentTemplate, error) {
	var t ComponentTemplate
	if err := m.db.WithContext(ctx).Where("key = ?", strings.TrimSpace(key)).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns templates filtered by category and scope.
func (m *ComponentTemplateModel) List(ctx context.Context, opts ComponentTemplateListOptions) ([]ComponentTemplate, int64, error) {
	query := m.db.WithContext(ctx).Model(&ComponentTemplate{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.BuiltinOnly {
		query = query.Where("builtin = ?", true)
	}
	if opts.CreatedBy != "" {
		query = query.Where("created_by = ?", opts.CreatedBy)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	opts.Normalize()
	var items []ComponentTemplate
	err := query.Order("builtin DESC, updated_at DESC").
		Limit(opts.PageSize).Offset((opts.Page - 1) * opts.PageSize).
		Find(&items).Error
	return items, total, err
}

// Update applies partial updates.
func (m *ComponentTemplateModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&ComponentTemplate{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a non-builtin template.
func (m *ComponentTemplateModel) Delete(ctx context.Context, id uint) error {
	res := m.db.WithContext(ctx).Where("id = ? AND builtin = ?", id, false).Delete(&ComponentTemplate{})
	if res.RowsAffected == 0 {
		return ErrComponentTemplateBuiltinDelete
	}
	return res.Error
}

// ComponentTemplateListOptions filters templates.
type ComponentTemplateListOptions struct {
	PaginationOptions
	Category    string
	BuiltinOnly bool
	CreatedBy   string
}

// Sentinel errors.
var (
	ErrComponentTemplateKeyRequired   = errComponentTemplateKeyRequired
	ErrComponentTemplateBuiltinDelete = errComponentTemplateBuiltinDelete
)
