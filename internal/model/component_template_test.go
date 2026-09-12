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

// TestComputeTemplateDigest canonical 稳定性：同一逻辑内容不受原始 JSON
// 字节序（键序/空格）影响；内容变化 digest 变化；空/非法输入容错空串。
func TestComputeTemplateDigest(t *testing.T) {
	a := ComputeTemplateDigest(JSON(`[{"type":"fnTable","props":{"functionId":"player.list","span":12}}]`))
	b := ComputeTemplateDigest(JSON(`[ {"props": {"span": 12, "functionId": "player.list"}, "type": "fnTable"} ]`))
	assert.NotEmpty(t, a)
	assert.Equal(t, a, b, "same logical content must share one digest")

	c := ComputeTemplateDigest(JSON(`[{"type":"fnTable","props":{"functionId":"order.list"}}]`))
	assert.NotEqual(t, a, c, "content change must alter the digest")

	assert.Empty(t, ComputeTemplateDigest(JSON(``)))
	assert.Empty(t, ComputeTemplateDigest(JSON(`not json`)))
}

// TestComponentTemplateDigestWritePaths U11 三写路径全覆盖：Create /
// UpsertBuiltin（新建 + 更新）都自动落 digest；内容不变重写 digest 不变，
// 内容变化 digest 跟随。
func TestComponentTemplateDigestWritePaths(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)
	ctx := context.Background()

	tpl := sampleTemplate("digest-create", false)
	require.NoError(t, m.Create(ctx, tpl))
	got, err := m.FindByKey(ctx, "digest-create")
	require.NoError(t, err)
	assert.NotEmpty(t, got.Digest, "Create must persist the digest")
	assert.Equal(t, ComputeTemplateDigest(tpl.Tree), got.Digest)

	v1 := sampleTemplate("digest-builtin", true)
	require.NoError(t, m.UpsertBuiltin(ctx, v1))
	got1, err := m.FindByKey(ctx, "digest-builtin")
	require.NoError(t, err)
	assert.Equal(t, ComputeTemplateDigest(v1.Tree), got1.Digest, "UpsertBuiltin create must persist the digest")

	v2 := sampleTemplate("digest-builtin", true)
	v2.Tree = JSON(`[{"type":"fnForm","props":{"functionId":"mail.send"}}]`)
	require.NoError(t, m.UpsertBuiltin(ctx, v2))
	got2, err := m.FindByKey(ctx, "digest-builtin")
	require.NoError(t, err)
	assert.Equal(t, ComputeTemplateDigest(v2.Tree), got2.Digest, "UpsertBuiltin update must refresh the digest")
	assert.NotEqual(t, got1.Digest, got2.Digest)
}
