package svc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 写种子文件到临时目录，返回目录路径。
func writeSeedFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, DefaultMenusFilename), []byte(content), 0o644))
	return dir
}

func TestLoadSeedMenus(t *testing.T) {
	t.Run("文件缺失 → 禁用不报错", func(t *testing.T) {
		seeds, err := LoadSeedMenus(t.TempDir())
		require.NoError(t, err)
		assert.Nil(t, seeds)
	})

	t.Run("空数组 → 禁用不报错", func(t *testing.T) {
		dir := writeSeedFile(t, "[]")
		seeds, err := LoadSeedMenus(dir)
		require.NoError(t, err)
		assert.Nil(t, seeds)
	})

	t.Run("合法文件解析字段", func(t *testing.T) {
		dir := writeSeedFile(t, `[
			{"menuKey":"player","labels":{"zh-CN":"玩家","en-US":"Players"},"icon":"UserOutlined","sortOrder":10},
			{"menuKey":"audit","labels":{"zh-CN":"审计"},"isVisible":false}
		]`)
		seeds, err := LoadSeedMenus(dir)
		require.NoError(t, err)
		require.Len(t, seeds, 2)
		assert.Equal(t, "player", seeds[0].MenuKey)
		assert.Equal(t, "Players", seeds[0].Labels["en-US"])
		assert.Equal(t, 10, seeds[0].SortOrder)
		assert.True(t, seeds[0].IsVisibleOrDefault())
		// isVisible 显式 false 生效
		assert.False(t, seeds[1].IsVisibleOrDefault())
	})

	t.Run("非法 menuKey → 整体禁用", func(t *testing.T) {
		dir := writeSeedFile(t, `[{"menuKey":"1player","labels":{"zh-CN":"玩家"}}]`)
		_, err := LoadSeedMenus(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid menuKey")
	})

	t.Run("labels 全空 → 整体禁用", func(t *testing.T) {
		dir := writeSeedFile(t, `[{"menuKey":"player","labels":{"zh-CN":""}}]`)
		_, err := LoadSeedMenus(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty label")
	})

	t.Run("JSON 解析失败 → 报错", func(t *testing.T) {
		dir := writeSeedFile(t, `{not-json`)
		_, err := LoadSeedMenus(dir)
		require.Error(t, err)
	})
}

func TestMenuSeederEnsureSeeded(t *testing.T) {
	ctx := context.Background()

	newSeeder := func(t *testing.T, seeds []SeedMenuItem) (*MenuSeeder, *model.MenuItemModel, *gorm.DB) {
		t.Helper()
		db := setupTestDB(t)
		require.NoError(t, autoMigrate(db))
		mm := model.NewMenuItemModel(db)
		return NewMenuSeeder(mm, seeds), mm, db
	}

	seeds := []SeedMenuItem{
		{MenuKey: "player", Labels: map[string]string{"zh-CN": "玩家管理", "en-US": "Players"}, Icon: "UserOutlined", SortOrder: 10},
		{MenuKey: "operation", Labels: map[string]string{"zh-CN": "运营", "en-US": "Operations"}, SortOrder: 20},
	}

	t.Run("seeds 为空 → no-op", func(t *testing.T) {
		seeder, mm, _ := newSeeder(t, nil)
		assert.False(t, seeder.Enabled())
		seeder.EnsureSeeded(ctx, "g1", "dev")
		count, err := mm.CountByScope(ctx, "g1", "dev")
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("空 scope 首访种入；二次调用幂等", func(t *testing.T) {
		seeder, mm, _ := newSeeder(t, seeds)
		seeder.EnsureSeeded(ctx, "g1", "dev")

		items, err := mm.ListByScope(ctx, "g1", "dev")
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, "player", items[0].MenuKey)
		assert.Equal(t, 10, items[0].SortOrder)
		assert.Equal(t, "UserOutlined", items[0].Icon)
		assert.True(t, items[0].IsVisible)
		assert.Equal(t, "玩家管理", items[0].GetLabels()["zh-CN"])

		// 同 scope 二次调用：进程内已标记，不重复导入。
		seeder.EnsureSeeded(ctx, "g1", "dev")
		count, err := mm.CountByScope(ctx, "g1", "dev")
		require.NoError(t, err)
		assert.EqualValues(t, 2, count)
	})

	t.Run("scope 已有菜单 → 永不导入", func(t *testing.T) {
		seeder, mm, _ := newSeeder(t, seeds)
		// 用户先建了一条菜单
		existing := &model.MenuItem{GameID: "g2", Env: "prod", MenuKey: "custom", IsVisible: true}
		require.NoError(t, existing.SetLabels(map[string]string{"zh-CN": "自定义"}))
		require.NoError(t, mm.Create(ctx, existing))

		seeder.EnsureSeeded(ctx, "g2", "prod")
		items, err := mm.ListByScope(ctx, "g2", "prod")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "custom", items[0].MenuKey)
	})

	t.Run("scope 相互独立", func(t *testing.T) {
		seeder, mm, _ := newSeeder(t, seeds)
		seeder.EnsureSeeded(ctx, "g1", "dev")
		seeder.EnsureSeeded(ctx, "g1", "prod")
		for _, env := range []string{"dev", "prod"} {
			count, err := mm.CountByScope(ctx, "g1", env)
			require.NoError(t, err)
			assert.EqualValues(t, 2, count, "env=%s", env)
		}
	})

	t.Run("nil seeder 调用安全", func(t *testing.T) {
		var seeder *MenuSeeder
		assert.False(t, seeder.Enabled())
		seeder.EnsureSeeded(ctx, "g1", "dev") // 不得 panic
	})

	t.Run("半套残留不再补种", func(t *testing.T) {
		// 上次种子中断只落了 1 条：count>0 → 本进程不再导入。
		seeder, mm, _ := newSeeder(t, seeds)
		partial := &model.MenuItem{GameID: "g3", Env: "dev", MenuKey: "player", IsVisible: true}
		require.NoError(t, partial.SetLabels(map[string]string{"zh-CN": "玩家管理"}))
		require.NoError(t, mm.Create(ctx, partial))

		seeder.EnsureSeeded(ctx, "g3", "dev")
		count, err := mm.CountByScope(ctx, "g3", "dev")
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
	})

	t.Run("种子文件是目录（读取失败）→ 报错禁用", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, DefaultMenusFilename), 0o755))
		_, err := LoadSeedMenus(dir)
		require.Error(t, err)
	})

	t.Run("种子内重复 menuKey → 撞唯一索引跳过该条继续", func(t *testing.T) {
		dupSeeds := []SeedMenuItem{
			{MenuKey: "player", Labels: map[string]string{"zh-CN": "玩家管理"}},
			{MenuKey: "player", Labels: map[string]string{"zh-CN": "玩家管理-重复"}},
		}
		seeder, mm, _ := newSeeder(t, dupSeeds)
		seeder.EnsureSeeded(ctx, "g4", "dev")
		// 第二条 create 撞唯一索引被跳过：只落第一条（labels 为首个定义）
		items, err := mm.ListByScope(ctx, "g4", "dev")
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "玩家管理", items[0].GetLabels()["zh-CN"])
	})

	t.Run("count 查询失败 → Warn 跳过不导入不 panic", func(t *testing.T) {
		seeder, _, db := newSeeder(t, seeds)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		seeder.EnsureSeeded(ctx, "g5", "dev") // 不得 panic
	})
}
