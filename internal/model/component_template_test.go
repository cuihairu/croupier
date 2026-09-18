package model

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// TestComponentTemplateDeleteHardDeleteRecreateSameKey 删除后同 key 重建：
// Delete 必须是硬删除——key 列的物理唯一索引会被软删行占位，重建 Create
// 直接 duplicate-key 500（线上实证过）。重建成功且库内不留残留行。
func TestComponentTemplateDeleteHardDeleteRecreateSameKey(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)
	ctx := context.Background()

	tpl := sampleTemplate("recreate-me", false)
	require.NoError(t, m.Create(ctx, tpl))
	require.NoError(t, m.Delete(ctx, tpl.ID))

	// 同 key 重建必须成功（软删残留会让这条 Create 撞唯一索引）。
	reborn := sampleTemplate("recreate-me", false)
	reborn.Category = "复盘"
	require.NoError(t, m.Create(ctx, reborn))
	got, err := m.FindByKey(ctx, "recreate-me")
	require.NoError(t, err)
	assert.Equal(t, "复盘", got.Category)

	// 硬删除不留残留：全表（含软删行）只有重建后的这一行。
	var rawCount int64
	require.NoError(t, db.Unscoped().Model(&ComponentTemplate{}).Where("key = ?", "recreate-me").Count(&rawCount).Error)
	assert.Equal(t, int64(1), rawCount, "soft-deleted residue must not linger")
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

// TestUpsertBuiltinContentGate 卡点 6：builtin 行内容未变时 UpsertBuiltin
// 跳写（updated_at 不刷新）；JSON 键序/空白不同语义相同视为未变；Tree/
// Name 变化正常写；非法 JSON 回退字节比较不误跳过；custom 占 key 行不
// 走门控维持覆盖语义。
func TestUpsertBuiltinContentGate(t *testing.T) {
	db := setupCompTplDB(t)
	m := NewComponentTemplateModel(db)
	ctx := context.Background()

	require.NoError(t, m.UpsertBuiltin(ctx, sampleTemplate("gate-mgmt", true)))
	got1, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	// 同内容二次 upsert：跳写，updated_at 不变。
	require.NoError(t, m.UpsertBuiltin(ctx, sampleTemplate("gate-mgmt", true)))
	got2, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)
	assert.True(t, got1.UpdatedAt.Equal(got2.UpdatedAt), "unchanged content must skip the write")

	time.Sleep(50 * time.Millisecond)
	// JSON 键序/空白不同但语义相同：跳写。
	reordered := sampleTemplate("gate-mgmt", true)
	reordered.Name = JSON(`{ "zh-CN" : "玩家管理" }`)
	reordered.Tree = JSON(`[ { "props" : { "functionId" : "player.list" } , "type" : "fnTable" } ]`)
	require.NoError(t, m.UpsertBuiltin(ctx, reordered))
	got3, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)
	assert.True(t, got2.UpdatedAt.Equal(got3.UpdatedAt), "key-order/whitespace-only differences are not content changes")

	time.Sleep(50 * time.Millisecond)
	// Tree 内容变化：写，digest 跟随刷新。
	changed := sampleTemplate("gate-mgmt", true)
	changed.Tree = JSON(`[{"type":"fnForm","props":{"functionId":"mail.send"}}]`)
	require.NoError(t, m.UpsertBuiltin(ctx, changed))
	got4, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)
	assert.True(t, got4.UpdatedAt.After(got3.UpdatedAt), "tree change must write")
	assert.Equal(t, ComputeTemplateDigest(changed.Tree), got4.Digest)

	time.Sleep(50 * time.Millisecond)
	// Name 变化：写。
	renamed := sampleTemplate("gate-mgmt", true)
	renamed.Name = JSON(`{"zh-CN":"玩家管理Pro"}`)
	require.NoError(t, m.UpsertBuiltin(ctx, renamed))
	got5, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)
	assert.True(t, got5.UpdatedAt.After(got4.UpdatedAt), "name change must write")
	assert.Equal(t, renamed.Name, got5.Name)

	time.Sleep(50 * time.Millisecond)
	// 非法 JSON：解析失败回退字节比较，不误跳过。
	broken := sampleTemplate("gate-mgmt", true)
	broken.Tree = JSON(`not json at all`)
	require.NoError(t, m.UpsertBuiltin(ctx, broken))
	got6, err := m.FindByKey(ctx, "gate-mgmt")
	require.NoError(t, err)
	assert.True(t, got6.UpdatedAt.After(got5.UpdatedAt), "unparseable JSON must fall back to byte comparison")
	assert.Equal(t, broken.Tree, got6.Tree)

	// custom 行占 key：不走门控（existing.Builtin=false），内容被覆盖
	// 且 Builtin 保持 false（现状语义：UpsertBuiltin 不翻转该标记）。
	customDB := setupCompTplDB(t)
	m2 := NewComponentTemplateModel(customDB)
	custom := sampleTemplate("gate-custom", false)
	custom.Category = "自定义"
	require.NoError(t, m2.Create(ctx, custom))
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, m2.UpsertBuiltin(ctx, sampleTemplate("gate-custom", true)))
	gotC, err := m2.FindByKey(ctx, "gate-custom")
	require.NoError(t, err)
	assert.False(t, gotC.Builtin, "UpsertBuiltin must not flip a custom row's builtin flag")
	assert.Equal(t, "运营", gotC.Category, "custom row occupying the key is still overwritten")
	assert.True(t, gotC.UpdatedAt.After(custom.UpdatedAt))
}
