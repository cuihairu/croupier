package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// MenuItem is a dashboard navigation entry. Menus are game-scoped like page
// specs (pages reference menus through PageSpec.MenuID) and support nesting
// via ParentID. Labels stores a LocalizedText JSON object keyed by BCP47
// locale ("zh-CN", "en-US").
type MenuItem struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	GameID     string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_menu_items_scope_key,priority:1;index:idx_menu_items_scope,priority:1" json:"gameId"`
	Env        string         `gorm:"size:64;not null;default:'';uniqueIndex:uidx_menu_items_scope_key,priority:2;index:idx_menu_items_scope,priority:2" json:"env"`
	ParentID   *uint          `gorm:"index" json:"parentId"`
	MenuKey    string         `gorm:"size:64;not null;uniqueIndex:uidx_menu_items_scope_key,priority:3" json:"menuKey"`
	Labels     string         `gorm:"column:labels;type:text" json:"-"`
	Icon       string         `gorm:"size:64" json:"icon,omitempty"`
	SortOrder  int            `gorm:"default:0" json:"sortOrder"`
	Permission string         `gorm:"size:128" json:"permission,omitempty"`
	// IsVisible 不带 default 标签：GORM 会对带 default 的字段在零值时省略
	// INSERT 列，导致显式 false 被数据库默认值 true 覆盖；可见性默认值由
	// service 层按「未提供即为 true」处理。
	IsVisible bool `gorm:"not null" json:"isVisible"`
}

func (MenuItem) TableName() string {
	return "menu_items"
}

// GetLabels returns the parsed labels LocalizedText.
func (m *MenuItem) GetLabels() map[string]string {
	var labels map[string]string
	if m.Labels != "" {
		_ = json.Unmarshal([]byte(m.Labels), &labels)
	}
	return labels
}

// SetLabels sets the labels from LocalizedText.
// map[string]string 的 json.Marshal 恒成功、无出错路径；error 返回值仅为
// 保持与 PageSpec.SetTitle 等既有模型助手签名一致而保留，恒返回 nil。
func (m *MenuItem) SetLabels(labels map[string]string) error {
	b, _ := json.Marshal(labels)
	m.Labels = string(b)
	return nil
}

// MenuItemModel provides data access for menu items.
type MenuItemModel struct {
	db *gorm.DB
}

// NewMenuItemModel creates a new MenuItemModel.
func NewMenuItemModel(db *gorm.DB) *MenuItemModel {
	return &MenuItemModel{db: db}
}

// FindByScopeAndKey returns a menu item by scope and menu_key.
func (m *MenuItemModel) FindByScopeAndKey(ctx context.Context, gameID, env, menuKey string) (*MenuItem, error) {
	var item MenuItem
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND menu_key = ?", gameID, env, menuKey).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByID returns a scoped menu item by ID.
func (m *MenuItemModel) FindByID(ctx context.Context, gameID, env string, id uint) (*MenuItem, error) {
	var item MenuItem
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND id = ?", gameID, env, id).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListByScope returns all menu items in a scope ordered for tree building.
func (m *MenuItemModel) ListByScope(ctx context.Context, gameID, env string) ([]MenuItem, error) {
	var items []MenuItem
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Create inserts a new menu item.
func (m *MenuItemModel) Create(ctx context.Context, item *MenuItem) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(item).Error
}

// Save updates an existing menu item.
func (m *MenuItemModel) Save(ctx context.Context, item *MenuItem) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Save(item).Error
}

// Delete hard-deletes a scoped menu item by ID. Hard delete by design: the
// scope+key unique index is physical, so a soft-deleted row would block
// recreating the same menuKey with a duplicate-key 500.
func (m *MenuItemModel) Delete(ctx context.Context, gameID, env string, id uint) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Unscoped().
		Where("game_id = ? AND env = ? AND id = ?", gameID, env, id).
		Delete(&MenuItem{}).Error
}

// DeleteByScopeAndKey hard-deletes a scoped menu item by menu_key.
func (m *MenuItemModel) DeleteByScopeAndKey(ctx context.Context, gameID, env, menuKey string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Unscoped().
		Where("game_id = ? AND env = ? AND menu_key = ?", gameID, env, menuKey).
		Delete(&MenuItem{}).Error
}

// CountByParent returns the number of direct children of a menu item.
func (m *MenuItemModel) CountByParent(ctx context.Context, gameID, env string, parentID uint) (int64, error) {
	var count int64
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&MenuItem{}).
		Where("game_id = ? AND env = ? AND parent_id = ?", gameID, env, parentID).
		Count(&count).Error
	return count, err
}

// UpdateSortOrder updates only the sort order of a scoped menu item.
func (m *MenuItemModel) UpdateSortOrder(ctx context.Context, gameID, env string, id uint, sortOrder int) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&MenuItem{}).
		Where("game_id = ? AND env = ? AND id = ?", gameID, env, id).
		Update("sort_order", sortOrder).Error
}
