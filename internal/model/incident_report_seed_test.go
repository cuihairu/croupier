package model

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func seedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&TaskSchedule{}, &TaskScheduleRunLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSeedIncidentReportSchedules_CreatesWeekAndMonth(t *testing.T) {
	db := seedTestDB(t)
	m := NewTaskScheduleModel(db)
	n, err := SeedIncidentReportSchedules(context.Background(), m, func(cronExpr string) (time.Time, bool) {
		return time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local), true
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n != 2 {
		t.Fatalf("seeded = %d, want 2", n)
	}
	for name, cron := range map[string]string{
		ScheduleNameIncidentWeek:  "0 9 * * 1",
		ScheduleNameIncidentMonth: "0 9 1 * *",
	} {
		var row TaskSchedule
		if err := db.Where("name = ?", name).First(&row).Error; err != nil {
			t.Fatalf("schedule %s missing: %v", name, err)
		}
		if row.CronExpr != cron {
			t.Fatalf("%s cron = %q, want %q", name, row.CronExpr, cron)
		}
		if row.Kind != ScheduleKindIncidentReport {
			t.Fatalf("%s kind = %q, want incident_report", name, row.Kind)
		}
		if row.FunctionID != FunctionIDIncidentReport {
			t.Fatalf("%s functionId = %q", name, row.FunctionID)
		}
		if row.GameID != "*" || row.Env != "*" {
			t.Fatalf("%s scope = %s/%s, want */*", name, row.GameID, row.Env)
		}
		if row.Status != ScheduleStatusActive {
			t.Fatalf("%s status = %q", name, row.Status)
		}
		if row.NextTriggeredAt == nil {
			t.Fatalf("%s next_triggered_at not set（ListDue 扫描条件）", name)
		}
		if row.Payload == nil || string(row.Payload) == "" || string(row.Payload) == "null" {
			t.Fatalf("%s payload empty", name)
		}
	}
}

func TestSeedIncidentReportSchedules_Idempotent(t *testing.T) {
	db := seedTestDB(t)
	m := NewTaskScheduleModel(db)
	next := func(string) (time.Time, bool) { return time.Now().Add(time.Hour), true }
	if _, err := SeedIncidentReportSchedules(context.Background(), m, next); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	// 用户改名/改 cron 的自定义行不被覆盖；重复播种零新增。
	if err := m.UpdateSchedule(context.Background(), 1, map[string]interface{}{"cron_expr": "0 10 * * 1"}); err != nil {
		t.Fatalf("customize: %v", err)
	}
	n, err := SeedIncidentReportSchedules(context.Background(), m, next)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if n != 0 {
		t.Fatalf("re-seed created %d rows, want 0", n)
	}
	var row TaskSchedule
	if err := db.Where("name = ?", ScheduleNameIncidentWeek).First(&row).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if row.CronExpr != "0 10 * * 1" {
		t.Fatalf("custom cron overwritten: %q", row.CronExpr)
	}
}

func TestSeedIncidentReportSchedules_MissingTableSkips(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// 无 AutoMigrate：表不存在（multiGame meta 库形态）——静默零播种。
	n, err := SeedIncidentReportSchedules(context.Background(), NewTaskScheduleModel(db), nil)
	if err != nil {
		t.Fatalf("seed on tableless db must not error: %v", err)
	}
	if n != 0 {
		t.Fatalf("seeded = %d, want 0", n)
	}
	// nil model 兜底不 panic。
	if n, err := SeedIncidentReportSchedules(context.Background(), nil, nil); err != nil || n != 0 {
		t.Fatalf("nil model: n=%d err=%v", n, err)
	}
}
