package model

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// MigrateTermDictionaryDisplay 把旧双列 display_zh/display_en 回填进
// display JSON 列（key 归一为 BCP47），随后删除旧列。
// 幂等：旧列不存在时直接返回。
//
// 实现说明：
//   - 逐行回填而非 SQL JSON 函数：SQLite/MySQL/Postgres/SQLServer 的
//     JSON 函数族不统一，Go 侧组装最可移植
//   - legacyRow 用 *string 承载 display 原始 JSON 文本而非 map——
//     GORM 对无 serializer 的 plain map 字段会报
//     "unsupported data type" 并跳过该字段
//   - 部分驱动（glebarez/sqlite）的 DropColumn 是静默 no-op：残留
//     旧列无害（模型不再映射），回填幂等（display 已是权威值），
//     因此删列失败不视为迁移失败
func MigrateTermDictionaryDisplay(db *gorm.DB) error {
	if db == nil || db.Migrator() == nil {
		return nil
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&TermDictionary{}) {
		return nil
	}

	hasZh := migrator.HasColumn(&TermDictionary{}, "display_zh")
	hasEn := migrator.HasColumn(&TermDictionary{}, "display_en")
	if !hasZh && !hasEn {
		return nil
	}

	selectCols := "id"
	if migrator.HasColumn(&TermDictionary{}, "display") {
		selectCols += ", display"
	}
	if hasZh {
		selectCols += ", display_zh"
	}
	if hasEn {
		selectCols += ", display_en"
	}

	type legacyRow struct {
		ID        uint
		Display   *string // 原始 JSON 文本，可能为 NULL
		DisplayZh string
		DisplayEn string
	}
	var rows []legacyRow
	if err := db.Table("term_dictionary").Select(selectCols).Find(&rows).Error; err != nil {
		return fmt.Errorf("scan legacy term_dictionary rows: %w", err)
	}

	for _, row := range rows {
		merged := map[string]string{}
		if row.Display != nil {
			merged = UnmarshalTermDisplayText(*row.Display)
		}
		for k, v := range merged {
			merged[NormalizeLocaleKey(k)] = v
		}
		if row.DisplayZh != "" {
			merged["zh-CN"] = row.DisplayZh
		}
		if row.DisplayEn != "" {
			merged["en-US"] = row.DisplayEn
		}
		normalized := NormalizeTermDisplay(merged)
		displayVal, err := marshalTermDisplay(normalized)
		if err != nil {
			return fmt.Errorf("serialize term_dictionary display id=%d: %w", row.ID, err)
		}
		if err := db.Table("term_dictionary").Where("id = ?", row.ID).
			Update("display", displayVal).Error; err != nil {
			return fmt.Errorf("backfill term_dictionary id=%d: %w", row.ID, err)
		}
	}

	// 删旧列用原生 ALTER TABLE 而非 migrator.DropColumn：GORM 的 sqlite
	// migrator 走"建新表-拷贝-改名"重建路径，实测会把 serializer 列的
	// 数据清空。原生 DROP COLUMN 在 SQLite 3.35+/MySQL/PG/SQLServer 均
	// 受支持；旧驱动不支持时降级保留旧列（无害：模型不再映射它们，
	// 回填幂等，display 已是权威值）。
	if hasZh {
		if err := db.Exec("ALTER TABLE term_dictionary DROP COLUMN display_zh").Error; err != nil {
			slog.Default().Warn("term_dictionary: drop legacy column display_zh failed, keeping it", "error", err)
		}
	}
	if hasEn {
		if err := db.Exec("ALTER TABLE term_dictionary DROP COLUMN display_en").Error; err != nil {
			slog.Default().Warn("term_dictionary: drop legacy column display_en failed, keeping it", "error", err)
		}
	}
	return nil
}
