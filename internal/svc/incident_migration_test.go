package svc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGoMigrations_IncidentTablesCatchUp 回归：已过 baseline 的存量库
// （无 incident_categories/incidents 表、bugs 无 category_id/subcategory
// 列、已有 bug 行）经 0040 catch-up 建表补列播种，存量行保留——线上
// postgres 只认编号迁移，模型加列 ≠ 迁移完成（0027 同族事故防线）。
func TestGoMigrations_IncidentTablesCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	// 落一行 0040 时代的 bug，再删两列 + 删两表模拟存量形态（sqlite 的
	// DropColumn 重建表但保留行数据）。
	if err := db.Exec(`INSERT INTO bugs (created_at, updated_at, title, content, severity, priority, status, source, assignee, created_by)
		VALUES (datetime('now'), datetime('now'), 'legacy crash', 'd', 'high', 'p1', 'triage', 'internal', 'alice', 'bob')`).Error; err != nil {
		t.Fatalf("seed legacy bug: %v", err)
	}
	if err := db.Migrator().DropColumn(&model.Bug{}, "CategoryID"); err != nil {
		t.Fatalf("drop bugs.category_id: %v", err)
	}
	if err := db.Migrator().DropColumn(&model.Bug{}, "Subcategory"); err != nil {
		t.Fatalf("drop bugs.subcategory: %v", err)
	}
	if err := db.Migrator().DropTable(&model.IncidentCategory{}); err != nil {
		t.Fatalf("drop incident_categories: %v", err)
	}
	if err := db.Migrator().DropTable(&model.Incident{}); err != nil {
		t.Fatalf("drop incidents: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil // baseline 已完成，禁止再跑 AutoMigrate
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasTable(&model.IncidentCategory{}) {
		t.Fatal("incident_categories table not created by 0040")
	}
	if !db.Migrator().HasTable(&model.Incident{}) {
		t.Fatal("incidents table not created by 0040")
	}
	for _, col := range []string{"CategoryID", "Subcategory"} {
		if !db.Migrator().HasColumn(&model.Bug{}, col) {
			t.Fatalf("bugs.%s not backfilled by 0040", col)
		}
	}
	// 播种：未分类 + 六类职能域共 7 行，全部 builtin
	var seeded []model.IncidentCategory
	if err := db.Where("1 = 1").Order("sort ASC").Find(&seeded).Error; err != nil {
		t.Fatalf("read seeds: %v", err)
	}
	if len(seeded) != 7 {
		t.Fatalf("seeded categories = %d, want 7", len(seeded))
	}
	if seeded[0].Slug != model.IncidentCatUncategorized || !seeded[0].Builtin {
		t.Fatalf("first seed = %+v, want builtin uncategorized", seeded[0])
	}
	// 存量 bug 行保留且补列后可写（缺列即断的写路径复证）
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM bugs WHERE title = 'legacy crash'").Scan(&count).Error; err != nil {
		t.Fatalf("read legacy bug: %v", err)
	}
	if count != 1 {
		t.Fatalf("legacy bug row lost: %d", count)
	}
	if err := db.Exec("UPDATE bugs SET category_id = ?, subcategory = '崩溃' WHERE title = 'legacy crash'", seeded[2].ID).Error; err != nil {
		t.Fatalf("update legacy bug category: %v", err)
	}
}

// TestMigrateIncidentTablesIdempotent：表列均已存在时 0040 跳过且播种
// 不重复（按 slug 缺行才插）；空库不报错（新表照常创建）。
func TestMigrateIncidentTablesIdempotent(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	if err := migrateIncidentTables(context.Background(), sqlDB); err != nil {
		t.Fatalf("empty db should not error, got %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m40.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.IncidentCategory{}, &model.Incident{}, &model.Bug{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB2, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateIncidentTables(context.Background(), sqlDB2); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var count int64
	if err := db.Model(&model.IncidentCategory{}).Count(&count).Error; err != nil {
		t.Fatalf("count seeds: %v", err)
	}
	if count != 7 {
		t.Fatalf("seeded categories = %d after re-run, want 7 (seed must be insert-if-missing)", count)
	}
}

// TestSeedIncidentCategoriesKeepsCustomRows：改过名的种子行（按 slug 命中）
// 不被回改；自定义类别不丢失——播种只补缺，不覆盖运营配置。
func TestSeedIncidentCategoriesKeepsCustomRows(t *testing.T) {
	db := openMigrationTestDB(t)
	if err := db.AutoMigrate(&model.IncidentCategory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	if err := model.SeedIncidentCategories(ctx, db); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	// 运营改名 + 新增自定义类别
	if err := db.Model(&model.IncidentCategory{}).Where("slug = ?", model.IncidentCatOps).
		Update("name", "基础设施组").Error; err != nil {
		t.Fatalf("rename ops: %v", err)
	}
	custom := &model.IncidentCategory{Name: "数据", Slug: "data", Enabled: true}
	if err := model.NewIncidentCategoryModel(db).Create(ctx, custom); err != nil {
		t.Fatalf("create custom: %v", err)
	}
	if err := model.SeedIncidentCategories(ctx, db); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	var opsRow model.IncidentCategory
	if err := db.Where("slug = ?", model.IncidentCatOps).First(&opsRow).Error; err != nil {
		t.Fatalf("read ops: %v", err)
	}
	if opsRow.Name != "基础设施组" {
		t.Fatalf("ops name = %q, rename must survive re-seed", opsRow.Name)
	}
	var total int64
	if err := db.Model(&model.IncidentCategory{}).Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 8 {
		t.Fatalf("total = %d, want 8 (7 seeds + 1 custom)", total)
	}
}
