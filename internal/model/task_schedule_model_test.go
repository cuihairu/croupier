package model

import (
	"context"
	"fmt"
	"sync/atomic"
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

	st, err := m.LastRunStatus(ctx, "demo", "prod", "run-1")
	require.NoError(t, err)
	assert.Equal(t, "", st) // TaskRun 未创建 → NotFound → 空

	// 空 taskRunID：直接空
	st2, err := m.LastRunStatus(ctx, "demo", "prod", "")
	require.NoError(t, err)
	assert.Equal(t, "", st2)
}

// ARCH-023 R30-04：跨槽陈旧领取扫描。ListStaleClaimsAfter 只返回
// status=claimed 且 created_at < before 的记录——非 claimed（已终态）不返回，
// 租约内（新近）不返回，limit 生效，cursor 游标分页生效。
func TestTaskScheduleModel_ListStaleClaimsAfter(t *testing.T) {
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
	stale, err := m.ListStaleClaimsAfter(ctx, before, 0, 100)
	require.NoError(t, err)
	require.Len(t, stale, 1, "只返回 created_at<before 且 status=claimed 的记录")
	assert.Equal(t, s1.ID, stale[0].ScheduleID)
	assert.Equal(t, oldSlot.UTC(), stale[0].Slot.UTC())
	assert.Equal(t, RunLogClaimed, stale[0].Status)

	// limit=1 仍命中 1 条；limit=0 不限制（同 1 条）。
	staleLimit, err := m.ListStaleClaimsAfter(ctx, before, 0, 1)
	require.NoError(t, err)
	assert.Len(t, staleLimit, 1)

	// 窗口收紧到 5 小时前：无陈旧领取 → 空。
	none, err := m.ListStaleClaimsAfter(ctx, now.Add(-5*time.Hour), 0, 100)
	require.NoError(t, err)
	assert.Len(t, none, 0)

	// cursor 分页：cursor=oldSlot 排除该条，命中空。
	afterCursor, err := m.ListStaleClaimsAfter(ctx, before, stale[0].ID, 100)
	require.NoError(t, err)
	assert.Len(t, afterCursor, 0, "cursor=已处理行 id 应排除已处理页")
}

// R30-05：UpdateRunLog 的 WHERE 带 status=claimed 再校验——领取→回填窗口内
// 另一实例已回写 dispatched（或本槽已终态）后，陈旧 reclaim 的 indeterminate
// 覆写必须 no-op，保留已落真实终值；claimed 行仍正常回填。
func TestTaskScheduleModel_UpdateRunLog_GuardNonClaimed(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	s, err := m.Create(ctx, CreateScheduleInput{
		Name: "c", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	slot := time.Now().UTC().Truncate(time.Minute)

	// 正常路径：claimed 行可回填（领取→派发）。
	ok, err := m.CreateRunLog(ctx, &TaskScheduleRunLog{ScheduleID: s.ID, Slot: slot, Status: RunLogClaimed})
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, m.UpdateRunLog(ctx, s.ID, slot, "run-1", RunLogDispatched, "ok"))
	got, err := m.GetRunLog(ctx, s.ID, slot)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, RunLogDispatched, got.Status)
	assert.Equal(t, "run-1", got.TaskRunID)

	// 竞态：已 dispatched（另一实例回填）后，陈旧 reclaim 覆写 indeterminate 必须 no-op。
	require.NoError(t, m.UpdateRunLog(ctx, s.ID, slot, "", RunLogIndeterminate, "claim lease expired; dispatch outcome unknown"))
	got2, err := m.GetRunLog(ctx, s.ID, slot)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, RunLogDispatched, got2.Status, "非 claimed 行不得被陈旧覆写")
	assert.Equal(t, "run-1", got2.TaskRunID)
}

// AUD-088：Create 恒产出 active 行，首触时间（FirstNext）必须与行同一次
// INSERT 落库——不留第二次 UPDATE 的窗口（update 失败即成 ListDue 永远扫不
// 出的死行）。
func TestTaskScheduleCreate_FirstNextSingleInsert(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	var inserts, updates atomic.Int64
	require.NoError(t, db.Callback().Create().After("gorm:create").Register("aud088_count_insert", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_schedules" {
			inserts.Add(1)
		}
	}))
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("aud088_count_update", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_schedules" {
			updates.Add(1)
		}
	}))

	next := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Second)
	s, err := m.Create(ctx, CreateScheduleInput{
		Name: "atomic", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod",
		FunctionID: "f", FirstNext: &next,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), inserts.Load(), "task_schedules 只允许一次 INSERT")
	assert.Equal(t, int64(0), updates.Load(), "Create 不得再触发 UPDATE")

	got, err := m.FindByID(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, ScheduleStatusActive, got.Status)
	require.NotNil(t, got.NextTriggeredAt, "首触时间必须随 INSERT 已落库")
	assert.True(t, got.NextTriggeredAt.UTC().Equal(next), "落库值必须等于 FirstNext，got=%v want=%v", got.NextTriggeredAt, next)

	// FirstNext=nil 保持旧形态（行落库后 NULL，由调用方自行回填——存量死行
	// 由 RepairMissingNextTrigger 收敛）。
	legacy, err := m.Create(ctx, CreateScheduleInput{
		Name: "legacy", CronExpr: "* * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	gotLegacy, err := m.FindByID(ctx, legacy.ID)
	require.NoError(t, err)
	assert.Nil(t, gotLegacy.NextTriggeredAt)
}

// AUD-088 存量收敛：active 且 next_triggered_at 为 NULL 的死行被盘点回填；
// paused+NULL 是暂停态的常态、dead_letter 恢复走 SetStatus 重算、健康行不动；
// cron 非法的行进 Unrepairable 保持原样待人工处置；收敛幂等。
func TestTaskScheduleModel_RepairMissingNextTrigger(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	// 死行：active + NULL（旧 insert-then-update 失败形态 / 播种回填失败形态）。
	deadRow, err := m.Create(ctx, CreateScheduleInput{
		Name: "dead", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	// 健康行：首触时间已随 INSERT 落库。
	healthy := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	healthyRow, err := m.Create(ctx, CreateScheduleInput{
		Name: "healthy", CronExpr: "0 3 * * *", GameID: "demo", Env: "prod", FunctionID: "f", FirstNext: &healthy,
	})
	require.NoError(t, err)
	// cron 非法的死行：无法重算，只能交人工。
	brokenRow, err := m.Create(ctx, CreateScheduleInput{
		Name: "broken", CronExpr: "not-a-cron", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	// 暂停行：暂停即清空首触时间，属正常形态，不得回填。
	pausedRow, err := m.Create(ctx, CreateScheduleInput{
		Name: "paused", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&TaskSchedule{}).Where("id = ?", pausedRow.ID).
		Updates(map[string]interface{}{"status": ScheduleStatusPaused, "next_triggered_at": nil}).Error)
	// 死信行：恢复走 SetStatus(active) 重算，不在收敛范围。
	deadLetterRow, err := m.Create(ctx, CreateScheduleInput{
		Name: "deadletter", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&TaskSchedule{}).Where("id = ?", deadLetterRow.ID).
		Update("status", ScheduleStatusDeadLetter).Error)

	next := time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC)
	res, err := m.RepairMissingNextTrigger(ctx, func(cronExpr string) (time.Time, bool) {
		if cronExpr == "not-a-cron" {
			return time.Time{}, false
		}
		return next, true
	})
	require.NoError(t, err)
	require.Len(t, res.Repaired, 1, "只回填 active+NULL 的死行")
	assert.Equal(t, deadRow.ID, res.Repaired[0].ID)
	require.NotNil(t, res.Repaired[0].NextTriggeredAt)
	assert.True(t, res.Repaired[0].NextTriggeredAt.Equal(next))
	require.Len(t, res.Unrepairable, 1, "cron 非法的死行进 Unrepairable")
	assert.Equal(t, brokenRow.ID, res.Unrepairable[0].ID)

	// 落库核验：死行已回填，其余各行保持原样。
	gotDead, err := m.FindByID(ctx, deadRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotDead.NextTriggeredAt)
	assert.True(t, gotDead.NextTriggeredAt.UTC().Equal(next))
	gotBroken, err := m.FindByID(ctx, brokenRow.ID)
	require.NoError(t, err)
	assert.Nil(t, gotBroken.NextTriggeredAt, "非法行保持原样待人工处置")
	assert.Equal(t, ScheduleStatusActive, gotBroken.Status)
	gotHealthy, err := m.FindByID(ctx, healthyRow.ID)
	require.NoError(t, err)
	require.NotNil(t, gotHealthy.NextTriggeredAt)
	assert.True(t, gotHealthy.NextTriggeredAt.UTC().Equal(healthy), "健康行首触时间不得被改写")
	gotPaused, err := m.FindByID(ctx, pausedRow.ID)
	require.NoError(t, err)
	assert.Nil(t, gotPaused.NextTriggeredAt, "暂停态 NULL 是正常形态，不得回填")
	assert.Equal(t, ScheduleStatusPaused, gotPaused.Status)
	gotDeadLetter, err := m.FindByID(ctx, deadLetterRow.ID)
	require.NoError(t, err)
	assert.Nil(t, gotDeadLetter.NextTriggeredAt, "死信行不在收敛范围")

	// 幂等：收敛完成后死行清零，非法行仍留在 Unrepairable 清单里。
	res2, err := m.RepairMissingNextTrigger(ctx, func(cronExpr string) (time.Time, bool) {
		if cronExpr == "not-a-cron" {
			return time.Time{}, false
		}
		return next, true
	})
	require.NoError(t, err)
	assert.Empty(t, res2.Repaired, "二次收敛不得重复回填")
	assert.Len(t, res2.Unrepairable, 1)
}

// AUD-088 竞争守卫：nextAfter 计算窗口内该行被并发暂停——回填 UPDATE 带
// status=active AND next_triggered_at IS NULL 再校验，RowsAffected=0 时按竞争
// 丢弃（不算 repaired），不覆盖已暂停的行。
func TestTaskScheduleModel_RepairMissingNextTrigger_RaceGuard(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	row, err := m.Create(ctx, CreateScheduleInput{
		Name: "race", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)

	next := time.Now().UTC().Add(10 * time.Minute)
	res, err := m.RepairMissingNextTrigger(ctx, func(cronExpr string) (time.Time, bool) {
		// 模拟扫描后、回填前的并发暂停（暂停态清空首触时间是设计行为）。
		require.NoError(t, db.Model(&TaskSchedule{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{"status": ScheduleStatusPaused, "next_triggered_at": nil}).Error)
		return next, true
	})
	require.NoError(t, err)
	assert.Empty(t, res.Repaired, "并发已暂停的行不得算作 repaired")
	got, err := m.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, ScheduleStatusPaused, got.Status, "回填不得覆盖并发状态变更")
	assert.Nil(t, got.NextTriggeredAt)
}

// AUD-088：nextAfter 为 nil 时全部死行进 Unrepairable（先例：播种路径允许
// 不注入回调），不产生写入。
func TestTaskScheduleModel_RepairMissingNextTrigger_NilCallback(t *testing.T) {
	db := setupTaskSchedDB(t)
	m := NewTaskScheduleModel(db)
	ctx := context.Background()

	row, err := m.Create(ctx, CreateScheduleInput{
		Name: "nil-cb", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "f",
	})
	require.NoError(t, err)

	res, err := m.RepairMissingNextTrigger(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Repaired)
	require.Len(t, res.Unrepairable, 1)
	assert.Equal(t, row.ID, res.Unrepairable[0].ID)
	got, err := m.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Nil(t, got.NextTriggeredAt)
}
