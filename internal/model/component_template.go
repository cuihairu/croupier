package model

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// Digest 模板内容指纹（sha256(canonical Tree JSON)，U11 更新提醒）：
	// 页面级快照记录实例化时的 digest，编辑器打开时与当前值比对提示新版本。
	Digest string `gorm:"size:64"`
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

// ComputeTemplateDigest 返回模板 Tree 的 canonical 内容指纹：
// unmarshal → marshal（map 键序稳定）后 sha256——同一逻辑内容不受
// 原始 JSON 字节序（空格/键序）影响。解析失败返回空串。
func ComputeTemplateDigest(tree JSON) string {
	if len(tree) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(tree, &value); err != nil {
		return ""
	}
	// value 是 json.Unmarshal 的产物，仅含 JSON 基础类型
	//（float64/string/bool/nil/map/slice），对其 Marshal 恒成功，err 分支为死代码已删。
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// jsonEquivalent 报告两段 JSON 的语义是否等价：unmarshal → marshal
// 规范化（与 ComputeTemplateDigest 同源，map 键序稳定、空白消除）后
// 比较。字节相同直接等价；任一侧解析失败且字节不同则不等价——保守
// 取向是宁误写不误跳过。
func jsonEquivalent(a, b JSON) bool {
	if bytes.Equal([]byte(a), []byte(b)) {
		return true
	}
	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	ra, _ := json.Marshal(va)
	rb, _ := json.Marshal(vb)
	return bytes.Equal(ra, rb)
}

// builtinContentEqual 报告 builtin 模板内容是否等价（UpsertBuiltin 跳写
// 门控）：只比对一次写入会真正覆盖的列——JSON 列（Name/Description/
// RequiredFunctions/Tree）走 jsonEquivalent，标量列（Category/Icon）直比。
// Params 不在 UpsertBuiltin 覆盖范围也不进门控；Digest/CreatedBy/时间戳
// 是推导值或元数据，均不参与。
func builtinContentEqual(existing, incoming *ComponentTemplate) bool {
	if existing == nil || incoming == nil {
		return false
	}
	if existing.Category != incoming.Category || existing.Icon != incoming.Icon {
		return false
	}
	return jsonEquivalent(existing.Name, incoming.Name) &&
		jsonEquivalent(existing.Description, incoming.Description) &&
		jsonEquivalent(existing.RequiredFunctions, incoming.RequiredFunctions) &&
		jsonEquivalent(existing.Tree, incoming.Tree)
}

// Create inserts a new template.
func (m *ComponentTemplateModel) Create(ctx context.Context, t *ComponentTemplate) error {
	t.Key = strings.TrimSpace(t.Key)
	if t.Key == "" {
		return ErrComponentTemplateKeyRequired
	}
	t.Digest = ComputeTemplateDigest(t.Tree)
	return m.db.WithContext(ctx).Create(t).Error
}

// UpsertBuiltin inserts or updates a builtin template by key.
func (m *ComponentTemplateModel) UpsertBuiltin(ctx context.Context, t *ComponentTemplate) error {
	t.Key = strings.TrimSpace(t.Key)
	if t.Key == "" {
		return ErrComponentTemplateKeyRequired
	}
	t.Builtin = true
	t.Digest = ComputeTemplateDigest(t.Tree)
	var existing ComponentTemplate
	err := m.db.WithContext(ctx).Where("key = ?", t.Key).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return m.db.WithContext(ctx).Create(t).Error
	}
	if err != nil {
		return err
	}
	// 内容门控（卡点 6）：builtin 行内容未变时跳写——逐函数注册风暴触发
	// 的全量模板重建不再无意义刷新 updated_at。custom 占 key 行不走门控
	// （existing.Builtin=false），维持覆盖语义。
	if existing.Builtin && builtinContentEqual(&existing, t) {
		return nil
	}
	existing.Name = t.Name
	existing.Description = t.Description
	existing.Category = t.Category
	existing.Icon = t.Icon
	existing.RequiredFunctions = t.RequiredFunctions
	existing.Tree = t.Tree
	existing.Digest = t.Digest
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

// Delete removes a non-builtin template. Hard delete: the key unique index is
// physical, so a soft-deleted row would block recreating the same key with a
// duplicate-key 500.
func (m *ComponentTemplateModel) Delete(ctx context.Context, id uint) error {
	res := m.db.WithContext(ctx).Unscoped().Where("id = ? AND builtin = ?", id, false).Delete(&ComponentTemplate{})
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
