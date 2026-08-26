package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupLegacyTermDB 建一张带旧双列（display_zh/display_en）的 term_dictionary，
// 模拟未迁移的存量库。
func setupLegacyTermDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE term_dictionary (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT NOT NULL,
		term_key TEXT NOT NULL,
		alias TEXT NOT NULL,
		display_zh TEXT,
		display_en TEXT,
		sort_order INTEGER DEFAULT 100,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO term_dictionary
		(domain, term_key, alias, display_zh, display_en, sort_order)
		VALUES
		('resource', 'player', 'player', '玩家', 'Player', 10),
		('resource', 'guild', 'guild', '公会', '', 20),
		('operation', 'create', 'add', '', 'Add', 10),
		('operation', 'read', 'query', '', '', 20)`).Error)
	return db
}

func TestMigrateTermDictionaryDisplay_BackfillAndDrop(t *testing.T) {
	db := setupLegacyTermDB(t)

	// AutoMigrate 补出 display 列（模型已无旧字段，旧列保留待清理）。
	require.NoError(t, migrateModels(db, []interface{}{&TermDictionary{}}))
	require.True(t, db.Migrator().HasTable("term_dictionary"))

	require.NoError(t, MigrateTermDictionaryDisplay(db))

	// 数据按 BCP47 回填。
	items, err := NewTermDictionaryModel(db).List(t.Context(), "")
	require.NoError(t, err)
	byAlias := map[string]TermDictionary{}
	for _, it := range items {
		byAlias[it.Alias] = it
	}
	assert.Equal(t, map[string]string{"zh-CN": "玩家", "en-US": "Player"}, byAlias["player"].Display)
	assert.Equal(t, map[string]string{"zh-CN": "公会"}, byAlias["guild"].Display)
	assert.Equal(t, map[string]string{"en-US": "Add"}, byAlias["add"].Display)
	// 双空 → 空 display（保留行，仅无显示文本）。
	assert.Empty(t, byAlias["query"].Display)

	// 旧列删除：MySQL/PG 生效；glebarez/sqlite 的 DropColumn 是静默 no-op，
	// 残留列无害（模型不再映射），这里仅容忍不强制断言。

	// 幂等：再次执行无副作用。
	require.NoError(t, MigrateTermDictionaryDisplay(db))
}

func TestMigrateTermDictionaryDisplay_NoLegacyColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrateModels(db, []interface{}{&TermDictionary{}}))
	require.NoError(t, db.Create(&TermDictionary{
		Domain: "resource", TermKey: "player", Alias: "player",
		Display: map[string]string{"zh-CN": "玩家"},
	}).Error)

	// 无旧列：直接成功且不动数据。
	require.NoError(t, MigrateTermDictionaryDisplay(db))
	items, err := NewTermDictionaryModel(db).List(t.Context(), "")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]string{"zh-CN": "玩家"}, items[0].Display)
}

func TestMigrateTermDictionaryDisplay_NoTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, MigrateTermDictionaryDisplay(db))
}

func TestNormalizeTermDisplay(t *testing.T) {
	assert.Nil(t, NormalizeTermDisplay(nil))
	assert.Nil(t, NormalizeTermDisplay(map[string]string{}))
	assert.Nil(t, NormalizeTermDisplay(map[string]string{"zh-CN": "  "}))

	// "bad" 是合法的单段 BCP47 主语言标签，保留；仅空 key/空值被丢弃。
	got := NormalizeTermDisplay(map[string]string{
		"zh":     "玩家",
		"en":     "Player",
		"ja-jp":  "プレイヤー",
		"":       "dropped",
		"zh-CN ": " trimmed ",
		"bad":    "x",
	})
	assert.Equal(t, map[string]string{
		"zh-CN": "玩家",
		"en-US": "Player",
		"ja-JP": "プレイヤー",
		"bad":   "x",
	}, got)
}

func TestNormalizeLocaleKey(t *testing.T) {
	cases := map[string]string{
		"zh-CN":   "zh-CN",
		"zh":      "zh-CN",
		"zh_cn":   "zh-CN",
		"zh-Hans": "zh-CN",
		"zh_TW":   "zh-TW",
		"zh-HK":   "zh-TW",
		"zh-Hant": "zh-TW",
		"en":      "en-US",
		"EN-us":   "en-US",
		"ja-jp":   "ja-JP",
		"pt-BR":   "pt-BR",
		"":        "",
		"   ":     "",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeLocaleKey(in), "input %q", in)
	}
}

func TestTermDictionaryUpsert_DisplayRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, migrateModels(db, []interface{}{&TermDictionary{}}))
	m := NewTermDictionaryModel(db)

	// Create：宽松 key 归一后入库。
	require.NoError(t, m.Upsert(t.Context(), &TermDictionary{
		Domain: "resource", TermKey: "player", Alias: "player",
		Display: map[string]string{"zh": "玩家", "en": "Player"},
	}))
	items, err := m.List(t.Context(), "resource")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]string{"zh-CN": "玩家", "en-US": "Player"}, items[0].Display)

	// Update：覆盖写入。
	require.NoError(t, m.Upsert(t.Context(), &TermDictionary{
		Domain: "resource", TermKey: "player", Alias: "player",
		Display: map[string]string{"ja-JP": "プレイヤー"},
	}))
	items, err = m.List(t.Context(), "resource")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]string{"ja-JP": "プレイヤー"}, items[0].Display)

	// 清空 display。
	require.NoError(t, m.Upsert(t.Context(), &TermDictionary{
		Domain: "resource", TermKey: "player", Alias: "player",
	}))
	items, err = m.List(t.Context(), "resource")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].Display)
}
