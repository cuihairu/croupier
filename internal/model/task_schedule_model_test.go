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
