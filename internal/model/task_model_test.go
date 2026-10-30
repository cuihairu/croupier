package model

import (
	"context"
	"testing"
	"time"

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
