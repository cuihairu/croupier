package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRunModel_ReapStale(t *testing.T) {
	db := newModelTestDB(t, &TaskRun{})
	m := NewTaskRunModel(db)
	ctx := context.Background()
	now := time.Now()

	seed := func(taskID, agentID, status string, updatedAt time.Time) {
		run := &TaskRun{
			TaskID:     taskID,
			FunctionID: "f",
			AgentID:    agentID,
			GameID:     "g",
			Env:        "e",
			Status:     status,
			StartedAt:  &now,
		}
		require.NoError(t, m.Create(ctx, run))
		require.NoError(t, db.Model(&TaskRun{}).Where("task_id = ?", taskID).
			Update("updated_at", updatedAt).Error)
	}

	// active+stale → timed_out（跨 agent 全局收割）。
	seed("t-stale-a", "agent-a", "running", now.Add(-10*time.Minute))
	seed("t-stale-b", "agent-b", "running", now.Add(-10*time.Minute))
	// fresh 不 reap。
	seed("t-fresh", "agent-a", "running", now.Add(-1*time.Minute))
	// terminal/unknown/timed_out 不 reap。
	seed("t-done", "agent-a", "succeeded", now.Add(-10*time.Minute))
	seed("t-unknown", "agent-a", "unknown", now.Add(-10*time.Minute))
	seed("t-timeout", "agent-a", "timed_out", now.Add(-10*time.Minute))
	// cancel_requested 收割。
	seed("t-cancel-req", "agent-a", "cancel_requested", now.Add(-10*time.Minute))

	since := now.Add(-5 * time.Minute)
	reaped, err := m.ReapStale(ctx, []string{"running", "cancel_requested", "queued", "dispatching"}, since, "timed_out", "no event past watchdog budget; execution state unknown")
	require.NoError(t, err)
	require.EqualValues(t, 3, reaped) // t-stale-a, t-stale-b, t-cancel-req

	// 收割行：状态/消息/finished_at 置位。
	for _, taskID := range []string{"t-stale-a", "t-stale-b", "t-cancel-req"} {
		run, err := m.FindByTaskID(ctx, taskID)
		require.NoError(t, err)
		require.Equal(t, "timed_out", run.Status, taskID)
		require.Equal(t, "no event past watchdog budget; execution state unknown", run.Message)
		require.NotNil(t, run.FinishedAt)
	}

	// 未收割行：状态原样（fresh 保持 running；terminal/unknown 不被触碰）。
	require.Equal(t, "running", func() string { r, _ := m.FindByTaskID(ctx, "t-fresh"); return r.Status }())
	require.Equal(t, "succeeded", func() string { r, _ := m.FindByTaskID(ctx, "t-done"); return r.Status }())
	require.Equal(t, "unknown", func() string { r, _ := m.FindByTaskID(ctx, "t-unknown"); return r.Status }())
	require.Equal(t, "timed_out", func() string { r, _ := m.FindByTaskID(ctx, "t-timeout"); return r.Status }())
	// terminal 行不受收割影响：message 不被覆写。
	timeoutRun, err := m.FindByTaskID(ctx, "t-timeout")
	require.NoError(t, err)
	require.Empty(t, timeoutRun.Message)

	// 空 activeStatuses no-op。
	n, err := m.ReapStale(ctx, nil, since, "timed_out", "msg")
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = m.ReapStale(ctx, []string{}, since, "timed_out", "msg")
	require.NoError(t, err)
	require.Zero(t, n)
}

// R30-03（ARCH-015）：RecordTaskEventTransactional 在单一事务内完成 guard 状态
// 更新与事件追加，成功路径事件按序落库，终态 guard 拦截时不追加事件。
func TestTaskRunModel_RecordTaskEventTransactional(t *testing.T) {
	db := newModelTestDB(t, &TaskRun{}, &TaskEvent{})
	m := NewTaskRunModel(db)
	em := NewTaskEventModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &TaskRun{TaskID: "t-1", FunctionID: "f", Status: "queued"}))
	terminal := []string{"succeeded", "failed", "cancelled", "timed_out"}

	// 首事件：guard 通过 → 状态更新 + seq=1。
	updated, err := m.RecordTaskEventTransactional(ctx, "t-1", terminal,
		map[string]interface{}{"status": "running", "progress": int32(50)},
		&TaskEvent{Type: "progress", Progress: 50, Message: "halfway"})
	require.NoError(t, err)
	require.True(t, updated)
	run, err := m.FindByTaskID(ctx, "t-1")
	require.NoError(t, err)
	require.Equal(t, "running", run.Status)
	require.EqualValues(t, 50, run.Progress)
	events, err := em.ListByTaskID(ctx, "t-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.EqualValues(t, 1, events[0].Seq)
	require.Equal(t, "halfway", events[0].Message)

	// 次事件：seq=2。
	updated, err = m.RecordTaskEventTransactional(ctx, "t-1", terminal,
		map[string]interface{}{"progress": int32(90)},
		&TaskEvent{Type: "progress", Progress: 90})
	require.NoError(t, err)
	require.True(t, updated)
	events, err = em.ListByTaskID(ctx, "t-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.EqualValues(t, 2, events[1].Seq)

	// 终态 guard 拦截：任务已终态 → updated=false，事件不追加。
	require.NoError(t, m.UpdateByTaskID(ctx, "t-1", map[string]interface{}{"status": "succeeded"}))
	updated, err = m.RecordTaskEventTransactional(ctx, "t-1", terminal,
		map[string]interface{}{"status": "running"},
		&TaskEvent{Type: "started"})
	require.NoError(t, err)
	require.False(t, updated)
	run, err = m.FindByTaskID(ctx, "t-1")
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)
	events, err = em.ListByTaskID(ctx, "t-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 2, "终态 guard 拦截后不得追加事件")

	// 非法入参。
	_, err = m.RecordTaskEventTransactional(ctx, "", terminal, map[string]interface{}{}, &TaskEvent{})
	require.Error(t, err)
	_, err = m.RecordTaskEventTransactional(ctx, "t-1", terminal, map[string]interface{}{}, nil)
	require.Error(t, err)
}

// R30-03（ARCH-015）：事件写失败时状态更新整体回滚——不产生「task_runs 已终态
// 但 task_events 缺失」的部分写入。注入方式：删除 task_events 表使追加失败，
// 事务应回滚状态更新。
func TestTaskRunModel_RecordTaskEventTransactional_RollsBackOnAppendFailure(t *testing.T) {
	db := newModelTestDB(t, &TaskRun{}, &TaskEvent{})
	m := NewTaskRunModel(db)
	ctx := context.Background()

	require.NoError(t, m.Create(ctx, &TaskRun{TaskID: "t-rb", FunctionID: "f", Status: "queued"}))
	require.NoError(t, db.Migrator().DropTable(&TaskEvent{}))

	now := time.Now()
	updated, err := m.RecordTaskEventTransactional(ctx, "t-rb",
		[]string{"succeeded", "failed", "cancelled", "timed_out"},
		map[string]interface{}{"status": "failed", "finished_at": &now},
		&TaskEvent{Type: "failed", Message: "boom"})
	require.Error(t, err, "事件表缺失必须报错（不得静默 ACK）")
	require.False(t, updated)

	// 状态更新必须随事务回滚：任务保持 queued，不出现 failed 部分写入。
	run, err := m.FindByTaskID(ctx, "t-rb")
	require.NoError(t, err)
	require.Equal(t, "queued", run.Status, "事件写失败必须回滚状态更新")
	assert.Nil(t, run.FinishedAt)
}
