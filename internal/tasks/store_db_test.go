package tasks

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTaskStore(t *testing.T) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskRun{}, &model.TaskEvent{}))
	return NewStore(model.NewTaskRunModel(db), model.NewTaskEventModel(db))
}

func newTaskRun(taskID string) *model.TaskRun {
	return &model.TaskRun{
		TaskID:     taskID,
		FunctionID: "fn-1",
		GameID:     "game1",
		Env:        "prod",
		Status:     StatusQueued,
	}
}

func TestStore_CreateAndGetRun(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	run := newTaskRun("task-1")
	require.NoError(t, store.CreateRun(ctx, run))

	got, err := store.GetRun(ctx, "task-1")
	require.NoError(t, err)
	assert.Equal(t, "task-1", got.TaskID)
	assert.Equal(t, StatusQueued, got.Status)
	assert.Equal(t, "fn-1", got.FunctionID)

	_, err = store.GetRun(ctx, "missing")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestStore_UpdateRun(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateRun(ctx, newTaskRun("task-2")))
	require.NoError(t, store.UpdateRun(ctx, "task-2", map[string]interface{}{
		"status":  StatusRunning,
		"message": "in flight",
	}))

	got, err := store.GetRun(ctx, "task-2")
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, got.Status)
	assert.Equal(t, "in flight", got.Message)
}

func TestStore_UpdateRunIfStatusNotIn(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateRun(ctx, newTaskRun("task-3")))

	// Non-terminal status: update applies.
	applied, err := store.UpdateRunIfStatusNotIn(ctx, "task-3", TerminalStatuses(), map[string]interface{}{
		"status": StatusRunning,
	})
	require.NoError(t, err)
	assert.True(t, applied)

	// Terminal status: update is blocked.
	require.NoError(t, store.UpdateRun(ctx, "task-3", map[string]interface{}{
		"status": StatusSucceeded,
	}))
	applied, err = store.UpdateRunIfStatusNotIn(ctx, "task-3", TerminalStatuses(), map[string]interface{}{
		"status": StatusRunning,
	})
	require.NoError(t, err)
	assert.False(t, applied)

	got, err := store.GetRun(ctx, "task-3")
	require.NoError(t, err)
	assert.Equal(t, StatusSucceeded, got.Status, "terminal status must not be rolled back")
}

func TestStore_UpdateRunIfStatusNotIn_MissingTask(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	applied, err := store.UpdateRunIfStatusNotIn(ctx, "no-such-task", TerminalStatuses(), map[string]interface{}{
		"status": StatusRunning,
	})
	require.NoError(t, err)
	assert.False(t, applied)
}

func TestStore_ListRuns_Filters(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	runA := newTaskRun("list-a")
	runA.Status = StatusRunning
	runB := newTaskRun("list-b")
	runB.Status = StatusSucceeded
	runB.GameID = "game2"
	runC := newTaskRun("list-c")
	runC.Env = "dev"
	for _, r := range []*model.TaskRun{runA, runB, runC} {
		require.NoError(t, store.CreateRun(ctx, r))
	}

	items, total, err := store.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 10},
		FunctionID:        "fn-1",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 3)

	items, total, err = store.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 10},
		Status:            StatusSucceeded,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "list-b", items[0].TaskID)

	items, total, err = store.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 10},
		GameID:            "game2",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "list-b", items[0].TaskID)

	items, total, err = store.ListRuns(ctx, model.ListTasksOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 10},
		Env:               "dev",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "list-c", items[0].TaskID)
}

func TestStore_AppendAndListEvents(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()
	require.NoError(t, store.CreateRun(ctx, newTaskRun("task-4")))

	require.NoError(t, store.AppendEvent(ctx, "task-4", EventQueued, 0, "queued", nil))
	require.NoError(t, store.AppendEvent(ctx, "task-4", EventStarted, 0, "started", []byte(`{"a":1}`)))
	require.NoError(t, store.AppendEvent(ctx, "task-4", EventProgress, 50, "halfway", nil))
	require.NoError(t, store.AppendEvent(ctx, "  task-4  ", EventLog, 0, "log", nil))

	events, err := store.ListEvents(ctx, "task-4", 0)
	require.NoError(t, err)
	require.Len(t, events, 4)
	assert.Equal(t, int64(1), events[0].Seq)
	assert.Equal(t, int64(4), events[3].Seq)
	assert.Equal(t, "queued", events[0].Type)
	assert.Equal(t, int32(50), events[2].Progress)
	assert.Equal(t, "halfway", events[2].Message)
	// Empty payload is normalized to JSON null.
	assert.Equal(t, "null", string(events[0].Payload))
	assert.JSONEq(t, `{"a":1}`, string(events[1].Payload))

	after, err := store.ListEvents(ctx, "task-4", 2)
	require.NoError(t, err)
	assert.Len(t, after, 2)
	assert.Equal(t, int64(3), after[0].Seq)
}

func TestStore_ListEvents_EmptyTaskID(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	events, err := store.ListEvents(ctx, "   ", 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestRuntime_ReportProgressAndLog(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()
	require.NoError(t, store.CreateRun(ctx, newTaskRun("task-5")))

	r := &Runtime{taskID: "task-5", ctx: ctx, store: store}
	require.NoError(t, r.ReportProgress(30, "thirty percent", []byte(`{"p":30}`)))
	require.NoError(t, r.Log("hello", nil))

	events, err := store.ListEvents(ctx, "task-5", 0)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, string(EventProgress), events[0].Type)
	assert.Equal(t, int32(30), events[0].Progress)
	assert.Equal(t, string(EventLog), events[1].Type)
	assert.Equal(t, int32(0), events[1].Progress)
	assert.Equal(t, "hello", events[1].Message)
}

func TestIsTerminalStatus(t *testing.T) {
	for _, s := range []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut} {
		assert.True(t, IsTerminalStatus(s), s)
	}
	for _, s := range []string{StatusQueued, StatusDispatching, StatusRunning, StatusCancelRequested, "", "unknown"} {
		assert.False(t, IsTerminalStatus(s), s)
	}
}

func TestTerminalStatuses(t *testing.T) {
	statuses := TerminalStatuses()
	assert.ElementsMatch(t, []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut}, statuses)

	// Mutating the returned slice must not corrupt shared state.
	statuses[0] = "mutated"
	assert.ElementsMatch(t, []string{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut}, TerminalStatuses())
}

func TestIsReapableStatus(t *testing.T) {
	assert.True(t, IsReapableStatus(StatusQueued))
	assert.True(t, IsReapableStatus(StatusDispatching))
	assert.True(t, IsReapableStatus(StatusRunning))
	assert.True(t, IsReapableStatus(StatusCancelRequested))
	assert.False(t, IsReapableStatus(StatusSucceeded))
	assert.False(t, IsReapableStatus(StatusFailed))
	assert.False(t, IsReapableStatus(StatusCancelled))
	assert.False(t, IsReapableStatus(StatusTimedOut))
	assert.False(t, IsReapableStatus(StatusUnknown))
	assert.False(t, IsReapableStatus(""))
}

// TestStore_ReapAgentNonTerminal（ARCH-014 D3）：断连收割只收敛该 agent 的
// 活动（非终态、非 unknown）任务到 unknown；终态与 unknown 行、以及其它
// agent 的行不动。
func TestStore_ReapAgentNonTerminal(t *testing.T) {
	store := setupTaskStore(t)
	ctx := context.Background()

	mk := func(id, agentID, status string) *model.TaskRun {
		r := newTaskRun(id)
		r.AgentID = agentID
		r.Status = status
		return r
	}
	require.NoError(t, store.CreateRun(ctx, mk("a-queued", "agentA", StatusQueued)))
	require.NoError(t, store.CreateRun(ctx, mk("a-running", "agentA", StatusRunning)))
	require.NoError(t, store.CreateRun(ctx, mk("a-succeeded", "agentA", StatusSucceeded)))
	require.NoError(t, store.CreateRun(ctx, mk("a-unknown", "agentA", StatusUnknown)))
	require.NoError(t, store.CreateRun(ctx, mk("b-running", "agentB", StatusRunning)))

	n, err := store.ReapAgentNonTerminal(ctx, "agentA", StatusUnknown, "agent session lost")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n) // a-queued + a-running

	for id, want := range map[string]string{
		"a-queued":    StatusUnknown,
		"a-running":   StatusUnknown,
		"a-succeeded": StatusSucceeded,
		"a-unknown":   StatusUnknown,
		"b-running":   StatusRunning,
	} {
		got, err := store.GetRun(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, want, got.Status, "task %s", id)
	}
	// a-running 收割后置了 finished_at（未知终态语义）。
	got, err := store.GetRun(ctx, "a-running")
	require.NoError(t, err)
	assert.NotNil(t, got.FinishedAt)
	assert.Equal(t, "agent session lost", got.Message)
}

func TestStore_ReapAgentNonTerminal_EmptyAgent(t *testing.T) {
	store := setupTaskStore(t)
	_, err := store.ReapAgentNonTerminal(context.Background(), "", StatusUnknown, "msg")
	assert.Error(t, err)
}
