package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMigrateCategoriesToMenusScript 用 sqlite 内存库真实执行
// scripts/migrate-categories-to-menus.sql，验证：菜单生成、页面挂载、
// 幂等重跑、一致性校验查询。
func TestMigrateCategoriesToMenusScript(t *testing.T) {
	scriptPath := filepath.Join("migrate-categories-to-menus.sql")
	raw, err := os.ReadFile(scriptPath)
	require.NoError(t, err, "迁移脚本必须存在且可读: %s", scriptPath)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migrate.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PageSpec{}, &model.MenuItem{}))

	seedLegacyPages(t, db)

	statements := splitSQLStatements(string(raw))
	require.GreaterOrEqual(t, len(statements), 2, "脚本应包含建菜单+挂载两条写入语句")

	// 首轮执行（写入语句 + 两条校验查询）
	for _, stmt := range statements[:2] {
		require.NoError(t, db.Exec(stmt).Error, "执行失败: %.80s", stmt)
	}

	// 校验一：无未挂载的分类页面
	var unmapped int64
	require.NoError(t, db.Raw(statements[2]).Scan(&unmapped).Error)
	assert.Zero(t, unmapped, "所有分类页面都应挂载到菜单")

	// 校验二：菜单 → 页面计数
	type menuCount struct {
		GameID    string
		Env       string
		MenuKey   string
		PageCount int64
	}
	var counts []menuCount
	require.NoError(t, db.Raw(statements[3]).Scan(&counts).Error)
	byKey := map[string]int64{}
	for _, c := range counts {
		byKey[c.GameID+"/"+c.Env+"/"+c.MenuKey] = c.PageCount
	}
	assert.EqualValues(t, 2, byKey["demo-game/development/resource"])
	assert.EqualValues(t, 1, byKey["demo-game/development/operation"])
	assert.EqualValues(t, 1, byKey["other-game/development/resource"])
	assert.Len(t, counts, 3, "三个 (game,env,category) 组合各生成一个菜单")

	// 抽查挂载正确性
	var playerPage model.PageSpec
	require.NoError(t, db.Where("page_key = ?", "resource--player").First(&playerPage).Error)
	require.NotNil(t, playerPage.MenuID)
	var menu model.MenuItem
	require.NoError(t, db.Where("game_id = ? AND env = ? AND menu_key = ?",
		"demo-game", "development", "resource").First(&menu).Error)
	assert.Equal(t, menu.ID, *playerPage.MenuID)
	// labels 取最近更新页面的值
	assert.JSONEq(t, `{"zh-CN":"资源管理","en-US":"Resource"}`, menu.Labels)
	assert.True(t, menu.IsVisible)
	assert.Equal(t, 1, menu.SortOrder, "sort_order 继承 category_order 最小值")

	// 幂等：重跑写入语句不产生重复菜单、不覆盖已有 menu_id
	for _, stmt := range statements[:2] {
		require.NoError(t, db.Exec(stmt).Error)
	}
	var menuTotal int64
	require.NoError(t, db.Model(&model.MenuItem{}).Count(&menuTotal).Error)
	assert.EqualValues(t, 3, menuTotal, "重跑不得产生重复菜单")
	require.NoError(t, db.Raw(statements[2]).Scan(&unmapped).Error)
	assert.Zero(t, unmapped)
}

func seedLegacyPages(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 存量库形态：旧列 category_labels_json 已不在 model 映射里
	// （T-M8 删除），迁移脚本目标库仍带该列，手工补建后用原生 SQL 播种。
	require.NoError(t, db.Exec("ALTER TABLE page_specs ADD COLUMN category_labels_json TEXT").Error)
	rows := []model.PageSpec{
		{
			GameID: "demo-game", Env: "development", PageKey: "resource--player",
			CategoryKey: "resource", CategoryOrder: 1, SpecJSON: `{}`,
		},
		{
			GameID: "demo-game", Env: "development", PageKey: "resource--order",
			CategoryKey: "resource", CategoryOrder: 2, SpecJSON: `{}`,
		},
		{
			GameID: "demo-game", Env: "development", PageKey: "operation--announce",
			CategoryKey: "operation", CategoryOrder: 2, SpecJSON: `{}`,
		},
		{
			GameID: "other-game", Env: "development", PageKey: "resource--leaderboard",
			CategoryKey: "resource", CategoryOrder: 1, SpecJSON: `{}`,
		},
		{
			// 无分类页面：不参与迁移
			GameID: "demo-game", Env: "development", PageKey: "uncategorized",
			CategoryKey: "", SpecJSON: `{}`,
		},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}
	labels := map[string]string{
		"resource--player":      `{"zh-CN":"资源管理","en-US":"Resource"}`,
		"resource--order":       `{"zh-CN":"资源管理(旧)","en-US":"Resource"}`,
		"operation--announce":   `{"zh-CN":"运营工具"}`,
		"resource--leaderboard": `{"zh-CN":"资源"}`,
	}
	for pageKey, labelJSON := range labels {
		require.NoError(t, db.Exec("UPDATE page_specs SET category_labels_json = ? WHERE page_key = ?", labelJSON, pageKey).Error)
	}
	// 让 resource 分类下“最新更新”的页面是 labels 正确的那个
	// （CURRENT_TIMESTAMP 秒级精度，同秒内会平局，显式拨后一天）
	require.NoError(t, db.Exec("UPDATE page_specs SET updated_at = datetime('now', '+1 day') WHERE page_key = 'resource--player'").Error)
}

// splitSQLStatements 按分号切分脚本（脚本保证语句内不含分号字面量）。
func splitSQLStatements(script string) []string {
	var out []string
	for _, stmt := range strings.Split(script, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}
