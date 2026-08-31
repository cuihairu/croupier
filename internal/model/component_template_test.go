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

var compTplSeq int

func setupCompTplDB(t *testing.T) *gorm.DB {
	t.Helper()
	compTplSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:ctpl%d?mode=memory&cache=shared", compTplSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ComponentTemplate{}))
	return db
}

func sampleTemplate(key string, builtin bool) *ComponentTemplate {
	return &ComponentTemplate{
		Key:               key,
		Name:              JSON(`{"zh-CN":"玩家管理"}`),
		Description:       JSON(`{"zh-CN":"搜索→详情→操作"}`),
		Category:          "运营",
		Icon:              "TeamOutlined",
		RequiredFunctions: JSON(`["player.list","player.get"]`),
		Tree:              JSON(`[{"type":"fnTable","props":{"functionId":"player.list"}}]`),
		Builtin:           builtin,
		CreatedBy:         "admin",
	}
}

// TestComponentTemplateCRUD 全生命周期。
func TestComponentTemplateCRUD(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)
	ctx := context.Background()

	// Create
	tpl := sampleTemplate("player-mgmt", false)
	require.NoError(t, m.Create(ctx, tpl))
	assert.NotZero(t, tpl.ID)

	// FindByKey
	got, err := m.FindByKey(ctx, "player-mgmt")
	require.NoError(t, err)
	assert.Equal(t, "运营", got.Category)

	// List with filter
	items, total, err := m.List(ctx, ComponentTemplateListOptions{Category: "运营"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)

	// Update
	require.NoError(t, m.Update(ctx, tpl.ID, map[string]interface{}{"icon": "UserOutlined"}))

	// Delete non-builtin
	require.NoError(t, m.Delete(ctx, tpl.ID))
	_, err = m.FindByKey(ctx, "player-mgmt")
	assert.Error(t, err)
}

// TestComponentTemplateBuiltinLifecycle 内置模板 upsert + 删除保护。
func TestComponentTemplateBuiltinLifecycle(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)
	ctx := context.Background()

	// UpsertBuiltin：新建
	v1 := sampleTemplate("resource-mgmt", true)
	require.NoError(t, m.UpsertBuiltin(ctx, v1))

	// UpsertBuiltin：更新（同 key）
	v2 := sampleTemplate("resource-mgmt", true)
	v2.Icon = "AppstoreOutlined"
	require.NoError(t, m.UpsertBuiltin(ctx, v2))

	got, err := m.FindByKey(ctx, "resource-mgmt")
	require.NoError(t, err)
	assert.Equal(t, "AppstoreOutlined", got.Icon)
	assert.True(t, got.Builtin)

	// 内置不可删除
	err = m.Delete(ctx, got.ID)
	assert.ErrorIs(t, err, ErrComponentTemplateBuiltinDelete)

	// BuiltinOnly 过滤
	items, _, err := m.List(ctx, ComponentTemplateListOptions{BuiltinOnly: true})
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// TestComponentTemplateValidation 空 key 拒绝。
func TestComponentTemplateValidation(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)

	err := m.Create(context.Background(), &ComponentTemplate{Key: "  "})
	assert.ErrorIs(t, err, ErrComponentTemplateKeyRequired)
}
