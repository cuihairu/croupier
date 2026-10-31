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

var taskSchedDBSeq int

func setupTaskSchedDB(t *testing.T) *gorm.DB {
	t.Helper()
	taskSchedDBSeq++
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:tsk%d?mode=memory&cache=shared", taskSchedDBSeq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TaskSchedule{}, &TaskScheduleRunLog{}, &TaskRun{}))
	return db
}

// 覆盖 TaskScheduleModel CRUD + 到期扫描 + 幂等运行日志（此前 0 用例）。

func TestTaskScheduleCRUD(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	in := CreateScheduleInput{
		Name: "批量发信", CronExpr: "* * * * *", GameID: "demo", Env: "prod",
		FunctionID: "mail.batch", Payload: JSON(`{"k":"v"}`),
	}
	s1, err := m.Create(ctx, in)
	require.NoError(t, err)
	assert.NotZero(t, s1.ID)

	got, err := m.FindByID(ctx, s1.ID)
	require.NoError(t, err)
	assert.Equal(t, "mail.batch", got.FunctionID)

	// List 过滤
	items, total, err := m.List(ctx, ListSchedulesOptions{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)

	// 更新与状态
	require.NoError(t, m.UpdateSchedule(ctx, s1.ID, map[string]interface{}{"cron_expr": "0 * * * *"}))
	require.NoError(t, m.SetStatus(ctx, s1.ID, "paused"))

	// FindByID 不存在
	_, err = m.FindByID(ctx, 999)
	assert.Error(t, err)

	// 删除
	require.NoError(t, m.Delete(ctx, s1.ID))
	_, total, _ = m.List(ctx, ListSchedulesOptions{GameID: "demo", Env: "prod"})
	assert.Zero(t, total)
}

func TestTaskScheduleValidation(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)

	_, err := m.Create(context.Background(), CreateScheduleInput{
		Name: "", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	assert.Error(t, err)

	_, err = m.Create(context.Background(), CreateScheduleInput{
		Name: "x", CronExpr: "", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	assert.Error(t, err)

	_, err = m.Create(context.Background(), CreateScheduleInput{
		Name: "x", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "",
	})
	assert.Error(t, err)
}

func TestTaskScheduleListDueAndRunLog(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	s1, err := m.Create(ctx, CreateScheduleInput{
		Name: "a", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)

	// 手动置 NextTriggeredAt 为过去 → 到期可扫出
	past := now.Add(-time.Minute)
	require.NoError(t, db.Model(&TaskSchedule{}).Where("id = ?", s1.ID).
		Update("next_triggered_at", past).Error)

	due, err := m.ListDue(ctx, now, 10)
	require.NoError(t, err)
	assert.Len(t, due, 1)

	// 运行日志：首次创建成功，同 slot 幂等拒绝
	slot := now.Truncate(time.Minute)
	first, err := m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s1.ID, Slot: slot, TaskRunID: "run-1", Status: "dispatched"})
	require.NoError(t, err)
	assert.True(t, first)

	has, err := m.HasRunLog(ctx, s1.ID, slot)
	require.NoError(t, err)
	assert.True(t, has)

	dup, err := m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s1.ID, Slot: slot, TaskRunID: "run-2"})
	require.NoError(t, err)
	assert.False(t, dup) // 幂等：同槽不重复

	st, err := m.LastRunStatus(ctx, "run-1")
	require.NoError(t, err)
	assert.Equal(t, "", st) // TaskRun 未创建 → NotFound → 空

	// 空 taskRunID：直接空
	st2, err := m.LastRunStatus(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "", st2)
}

// ARCH-023 R30-04：跨槽陈旧领取扫描。ListStaleClaims 只返回
// status=claimed 且 created_at < before 的记录——非 claimed（已终态）不返回，
// 租约内（新近）不返回，limit 生效。
func TestTaskScheduleModel_ListStaleClaims(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	s1, err := m.Create(ctx, CreateScheduleInput{
		Name: "a", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	s2, err := m.Create(ctx, CreateScheduleInput{
		Name: "b", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)

	// 旧 claimed：s1 在 2 小时前领取，崩溃遗留。
	oldSlot := now.Add(-2 * time.Hour).Truncate(time.Minute)
	ok, err := m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s1.ID, Slot: oldSlot, Status: RunLogClaimed})
	require.NoError(t, err)
	require.True(t, ok)
	// 手动把 created_at 拨老（CreateRunLog 填的是当前时间）。
	require.NoError(t, db.Model(&TaskScheduleRunLog{}).Where("schedule_id = ? AND slot = ?", s1.ID, oldSlot).
		Update("created_at", now.Add(-3*time.Hour)).Error)

	// 新 claimed（租约内）：s2 刚领取，不得返回。
	freshSlot := now.Truncate(time.Minute)
	ok, err = m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s2.ID, Slot: freshSlot, Status: RunLogClaimed})
	require.NoError(t, err)
	require.True(t, ok)

	// 已终态（dispatched）旧记录：s1 的另一旧槽，不得返回（非 claimed）。
	termSlot := now.Add(-4 * time.Hour).Truncate(time.Minute)
	ok, err = m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s1.ID, Slot: termSlot, Status: RunLogDispatched})
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, db.Model(&TaskScheduleRunLog{}).Where("schedule_id = ? AND slot = ?", s1.ID, termSlot).
		Update("created_at", now.Add(-5*time.Hour)).Error)

	// 窗口 before = now - lease（lease=60s）：只命中 s1 的旧 claimed。
	before := now.Add(-time.Minute)
	stale, err := m.ListStaleClaims(ctx, before, 100)
	require.NoError(t, err)
	require.Len(t, stale, 1, "只返回 created_at<before 且 status=claimed 的记录")
	assert.Equal(t, s1.ID, stale[0].ScheduleID)
	assert.Equal(t, oldSlot.UTC(), stale[0].Slot.UTC())
	assert.Equal(t, RunLogClaimed, stale[0].Status)

	// limit=1 仍命中 1 条；limit=0 不限制（同 1 条）。
	staleLimit, err := m.ListStaleClaims(ctx, before, 1)
	require.NoError(t, err)
	assert.Len(t, staleLimit, 1)

	// 窗口收紧到 5 小时前：无陈旧领取 → 空。
	none, err := m.ListStaleClaims(ctx, now.Add(-5*time.Hour), 100)
	require.NoError(t, err)
	assert.Len(t, none, 0)
}
