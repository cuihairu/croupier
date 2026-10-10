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

// TestGoMigrations_ReportDistributionCatchUp 回归：已过 baseline 的存量库
// （无 incident_reports/external_tokens 表、messages 无通知强化五列、
// task_schedules 无 kind 列、已有两表存量行）经 0041 catch-up 建表补列
// 回填，存量行保留且补列可写——模型加列 ≠ 迁移完成（0027 同族防线，
// docs/design/incident-reports.md §2.5）。
func TestGoMigrations_ReportDistributionCatchUp(t *testing.T) {
	db := openMigrationTestDB(t)
	ctx := context.Background()

	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate: %v", err)
	}
	// 落 0041 之前的存量形态：messages/task_schedules 各一行（只带旧列），
	// 再删新列 + 删两新表模拟存量库（sqlite 的 DropColumn 重建表但保留行）。
	if err := db.Exec(`INSERT INTO messages (created_at, updated_at, recipient, type, title, content, status)
		VALUES (datetime('now'), datetime('now'), 'alice', 'system', 'legacy notice', 'body', 0)`).Error; err != nil {
		t.Fatalf("seed legacy message: %v", err)
	}
	if err := db.Exec(`INSERT INTO task_schedules (created_at, updated_at, name, cron_expr, game_id, env, function_id, status)
		VALUES (datetime('now'), datetime('now'), 'legacy job', '0 9 * * 1', 'demo', 'prod', 'game.kick', 'active')`).Error; err != nil {
		t.Fatalf("seed legacy schedule: %v", err)
	}
	if err := db.Migrator().DropTable(&model.IncidentReport{}); err != nil {
		t.Fatalf("drop incident_reports: %v", err)
	}
	if err := db.Migrator().DropTable(&model.ExternalToken{}); err != nil {
		t.Fatalf("drop external_tokens: %v", err)
	}
	for _, col := range []string{"Level", "Source", "RefType", "RefID", "Scope"} {
		if err := db.Migrator().DropColumn(&model.Message{}, col); err != nil {
			t.Fatalf("drop messages.%s: %v", col, err)
		}
	}
	if err := db.Migrator().DropColumn(&model.TaskSchedule{}, "Kind"); err != nil {
		t.Fatalf("drop task_schedules.kind: %v", err)
	}
	if _, err := migrate.EnsureUpToDate(ctx, db, migrate.ScopeSingle, func(db *gorm.DB) error {
		return nil // baseline 已完成，禁止再跑 AutoMigrate
	}); err != nil {
		t.Fatalf("EnsureUpToDate: %v", err)
	}
	if !db.Migrator().HasTable(&model.IncidentReport{}) {
		t.Fatal("incident_reports table not created by 0041")
	}
	if !db.Migrator().HasTable(&model.ExternalToken{}) {
		t.Fatal("external_tokens table not created by 0041")
	}
	for _, col := range []string{"Level", "Source", "RefType", "RefID", "Scope"} {
		if !db.Migrator().HasColumn(&model.Message{}, col) {
			t.Fatalf("messages.%s not backfilled by 0041", col)
		}
	}
	if !db.Migrator().HasColumn(&model.TaskSchedule{}, "Kind") {
		t.Fatal("task_schedules.kind not backfilled by 0041")
	}
	// 回填口径：存量行 level=info / kind=function
	var level string
	if err := db.Raw("SELECT level FROM messages WHERE type = 'system'").Scan(&level).Error; err != nil {
		t.Fatalf("read legacy message level: %v", err)
	}
	if level != model.MessageLevelInfo {
		t.Fatalf("legacy message level = %q, want %q", level, model.MessageLevelInfo)
	}
	var kind string
	if err := db.Raw("SELECT kind FROM task_schedules WHERE name = 'legacy job'").Scan(&kind).Error; err != nil {
		t.Fatalf("read legacy schedule kind: %v", err)
	}
	if kind != model.ScheduleKindFunction {
		t.Fatalf("legacy schedule kind = %q, want %q", kind, model.ScheduleKindFunction)
	}
	// 补列后可写（缺列即断的写路径复证）
	if err := db.Exec("UPDATE messages SET ref_type = 'incident', ref_id = '7' WHERE type = 'system'").Error; err != nil {
		t.Fatalf("update legacy message refs: %v", err)
	}
	if err := db.Exec("UPDATE task_schedules SET kind = 'incident_report' WHERE name = 'legacy job'").Error; err != nil {
		t.Fatalf("update legacy schedule kind: %v", err)
	}
}

// TestMigrateReportDistributionIdempotent：空库不报错（新表照常创建）；
// 列已存在但值为空的行（模拟无 default 补列的历史形态）被回填，重复
// 执行不报错不重复改。
func TestMigrateReportDistributionIdempotent(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	if err := migrateReportDistribution(context.Background(), sqlDB); err != nil {
		t.Fatalf("empty db should not error, got %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "m41.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}, &model.TaskSchedule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 显式写空值绕过列 default，构造「补列时未回填」的历史形态
	if err := db.Exec(`INSERT INTO messages (created_at, updated_at, recipient, type, status, level)
		VALUES (datetime('now'), datetime('now'), 'bob', 'system', 0, '')`).Error; err != nil {
		t.Fatalf("seed empty-level message: %v", err)
	}
	if err := db.Exec(`INSERT INTO task_schedules (created_at, updated_at, name, cron_expr, game_id, env, function_id, status, kind)
		VALUES (datetime('now'), datetime('now'), 'empty kind', '0 9 * * 1', 'demo', 'prod', 'game.kick', 'active', '')`).Error; err != nil {
		t.Fatalf("seed empty-kind schedule: %v", err)
	}
	sqlDB2, err := db.DB()
	if err != nil {
		t.Fatalf("sql.DB: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateReportDistribution(context.Background(), sqlDB2); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var level string
	if err := db.Raw("SELECT level FROM messages WHERE recipient = 'bob'").Scan(&level).Error; err != nil {
		t.Fatalf("read level: %v", err)
	}
	if level != model.MessageLevelInfo {
		t.Fatalf("level = %q, want backfilled %q", level, model.MessageLevelInfo)
	}
	var kind string
	if err := db.Raw("SELECT kind FROM task_schedules WHERE name = 'empty kind'").Scan(&kind).Error; err != nil {
		t.Fatalf("read kind: %v", err)
	}
	if kind != model.ScheduleKindFunction {
		t.Fatalf("kind = %q, want backfilled %q", kind, model.ScheduleKindFunction)
	}
	var msgs, schedules int64
	if err := db.Model(&model.Message{}).Count(&msgs).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if msgs != 1 {
		t.Fatalf("messages = %d, want 1 (backfill must not duplicate rows)", msgs)
	}
	if err := db.Model(&model.TaskSchedule{}).Count(&schedules).Error; err != nil {
		t.Fatalf("count schedules: %v", err)
	}
	if schedules != 1 {
		t.Fatalf("task_schedules = %d, want 1", schedules)
	}
}
