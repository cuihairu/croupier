package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var menuDBSeq int

func setupMenuDB(t *testing.T) *gorm.DB {
	t.Helper()
	menuDBSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:menu_model%d?mode=memory&cache=shared", menuDBSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MenuItem{}))
	return db
}

func sampleMenuItem(menuKey string) *MenuItem {
	item := &MenuItem{
		GameID:     "demo-game",
		Env:        "development",
		MenuKey:    menuKey,
		Icon:       "DatabaseOutlined",
		SortOrder:  1,
		Permission: "resource:read",
		IsVisible:  true,
	}
	// SetLabels 恒成功（json.Marshal map 无出错路径），返回值可安全忽略。
	_ = item.SetLabels(map[string]string{"zh-CN": "资源管理", "en-US": "Resource"})
	return item
}

// TestMenuItemAutoMigrateColumns 校验 menu_items 表结构包含设计要求的全部列。
func TestMenuItemAutoMigrateColumns(t *testing.T) {
	db := setupMenuDB(t)
	require.True(t, db.Migrator().HasTable("menu_items"))
	for _, col := range []string{
		"id", "parent_id", "menu_key", "labels", "icon",
		"sort_order", "permission", "is_visible", "game_id", "env",
		"created_at", "updated_at", "deleted_at",
	} {
		assert.True(t, db.Migrator().HasColumn(&MenuItem{}, col), "missing column %s", col)
	}
}

// TestMenuItemAutoMigrateViaGameModels 校验 GameModels() 自动迁移会创建 menu_items 表。
func TestMenuItemAutoMigrateViaGameModels(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:menu_gamemodels_%d?mode=memory&cache=shared", menuDBSeq+1000)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, migrateModels(db, GameModels()))
	assert.True(t, db.Migrator().HasTable(&MenuItem{}))
}

// TestMenuItemCRUD 全生命周期：创建、按 key 查询、labels 往返、更新、删除。
func TestMenuItemCRUD(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	item := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, item))
	assert.NotZero(t, item.ID)
	assert.True(t, item.IsVisible)

	// 显式 false 可见性往返不被默认值覆盖
	hidden := sampleMenuItem("hidden")
	hidden.IsVisible = false
	require.NoError(t, m.Create(ctx, hidden))
	gotHidden, err := m.FindByScopeAndKey(ctx, "demo-game", "development", "hidden")
	require.NoError(t, err)
	assert.False(t, gotHidden.IsVisible)

	got, err := m.FindByScopeAndKey(ctx, "demo-game", "development", "resource")
	require.NoError(t, err)
	assert.Equal(t, "resource", got.MenuKey)
	assert.Equal(t, map[string]string{"zh-CN": "资源管理", "en-US": "Resource"}, got.GetLabels())
	assert.Equal(t, "DatabaseOutlined", got.Icon)
	assert.Equal(t, "resource:read", got.Permission)

	// scope 隔离：其他 game 查不到
	_, err = m.FindByScopeAndKey(ctx, "other-game", "development", "resource")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	got.SortOrder = 5
	require.NoError(t, m.Save(ctx, got))
	again, err := m.FindByID(ctx, "demo-game", "development", got.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, again.SortOrder)

	require.NoError(t, m.Delete(ctx, "demo-game", "development", again.ID))
	_, err = m.FindByID(ctx, "demo-game", "development", again.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestMenuItemUniqueScopeKey 同 scope 下 menu_key 唯一；跨 scope 可重名。
func TestMenuItemUniqueScopeKey(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	first := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, first))

	dup := sampleMenuItem("resource")
	assert.Error(t, m.Create(ctx, dup))

	other := sampleMenuItem("resource")
	other.GameID = "other-game"
	require.NoError(t, m.Create(ctx, other))
}

// TestMenuItemNesting 父子嵌套：ParentID 指向父菜单，CountByParent 统计子节点。
func TestMenuItemNesting(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	parent := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, parent))

	childKeys := []string{"player", "order"}
	for i, key := range childKeys {
		child := sampleMenuItem(key)
		child.ParentID = &parent.ID
		child.SortOrder = i + 1
		require.NoError(t, m.Create(ctx, child))
	}

	count, err := m.CountByParent(ctx, "demo-game", "development", parent.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	items, err := m.ListByScope(ctx, "demo-game", "development")
	require.NoError(t, err)
	require.Len(t, items, 3)
	// 父节点 sort_order=1 且 id 最小排第一；子节点 player(1) 在 order(2) 前
	assert.Equal(t, "resource", items[0].MenuKey)
	assert.Equal(t, "player", items[1].MenuKey)
	assert.Equal(t, "order", items[2].MenuKey)
	require.NotNil(t, items[1].ParentID)
	assert.Equal(t, parent.ID, *items[1].ParentID)
}

// TestMenuItemUpdateSortOrder 仅更新排序字段。
func TestMenuItemUpdateSortOrder(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	item := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, item))

	require.NoError(t, m.UpdateSortOrder(ctx, "demo-game", "development", item.ID, 99))
	got, err := m.FindByID(ctx, "demo-game", "development", item.ID)
	require.NoError(t, err)
	assert.Equal(t, 99, got.SortOrder)
	assert.Equal(t, "DatabaseOutlined", got.Icon, "other fields untouched")
}

// TestPageSpecMenuAssociation 页面 menu_id 挂载/解除/批量清理。
func TestPageSpecMenuAssociation(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:menu_page_assoc_%d?mode=memory&cache=shared", menuDBSeq+2000)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MenuItem{}, &PageSpec{}))
	m := NewMenuItemModel(db)
	pm := NewPageSpecModel(db)
	ctx := context.Background()

	menu := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, menu))
	page := &PageSpec{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "resource--player",
		SpecJSON: `{"pageKey":"resource--player"}`,
	}
	require.NoError(t, db.Create(page).Error)

	// 挂载
	menuID := menu.ID
	require.NoError(t, pm.UpdateMenuID(ctx, "demo-game", "development", "resource--player", &menuID))
	got, err := pm.FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	require.NotNil(t, got.MenuID)
	assert.Equal(t, menu.ID, *got.MenuID)

	// 解除
	require.NoError(t, pm.UpdateMenuID(ctx, "demo-game", "development", "resource--player", nil))
	got, err = pm.FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.Nil(t, got.MenuID)

	// 批量清理（挂回两个页面后按菜单清理）
	second := &PageSpec{
		GameID:   "demo-game",
		Env:      "development",
		PageKey:  "resource--order",
		SpecJSON: `{"pageKey":"resource--order"}`,
	}
	require.NoError(t, db.Create(second).Error)
	require.NoError(t, pm.UpdateMenuID(ctx, "demo-game", "development", "resource--player", &menuID))
	require.NoError(t, pm.UpdateMenuID(ctx, "demo-game", "development", "resource--order", &menuID))
	require.NoError(t, pm.ClearMenuReferences(ctx, "demo-game", "development", []uint{menu.ID}))
	got, err = pm.FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--player")
	require.NoError(t, err)
	assert.Nil(t, got.MenuID)
	got, err = pm.FindByScopeAndPageKey(ctx, "demo-game", "development", "resource--order")
	require.NoError(t, err)
	assert.Nil(t, got.MenuID)

	// 空 ID 列表是 no-op
	require.NoError(t, pm.ClearMenuReferences(ctx, "demo-game", "development", nil))
}

// TestMenuItemDeleteByScopeAndKey 按 menu_key 硬删除。
func TestMenuItemDeleteByScopeAndKey(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	item := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, item))

	require.NoError(t, m.DeleteByScopeAndKey(ctx, "demo-game", "development", "resource"))
	_, err := m.FindByScopeAndKey(ctx, "demo-game", "development", "resource")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 同 key 可重建（硬删除不占用唯一索引）
	recreated := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, recreated))
}

// TestMenuItemDBFailureBranches 关闭底层连接：各方法的 error 分支统一
// 返回错误而非 panic。
func TestMenuItemDBFailureBranches(t *testing.T) {
	db := setupMenuDB(t)
	m := NewMenuItemModel(db)
	ctx := context.Background()

	item := sampleMenuItem("resource")
	require.NoError(t, m.Create(ctx, item))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = m.FindByScopeAndKey(ctx, "demo-game", "development", "resource")
	assert.Error(t, err)
	_, err = m.FindByID(ctx, "demo-game", "development", item.ID)
	assert.Error(t, err)
	_, err = m.ListByScope(ctx, "demo-game", "development")
	assert.Error(t, err)
	_, err = m.CountByScope(ctx, "demo-game", "development")
	assert.Error(t, err)
	_, err = m.CountByParent(ctx, "demo-game", "development", item.ID)
	assert.Error(t, err)
	assert.Error(t, m.Create(ctx, sampleMenuItem("another")))
	assert.Error(t, m.Save(ctx, item))
	assert.Error(t, m.Delete(ctx, "demo-game", "development", item.ID))
	assert.Error(t, m.DeleteByScopeAndKey(ctx, "demo-game", "development", "resource"))
	assert.Error(t, m.UpdateSortOrder(ctx, "demo-game", "development", item.ID, 2))
}
