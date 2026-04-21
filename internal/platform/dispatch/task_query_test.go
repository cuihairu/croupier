package dispatch

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestTaskEventQueryAdapterGetRunMapsTaskRunState(t *testing.T) {
	t.Parallel()

	db := openDispatchTestDB(t)
	ctx := context.Background()
	runs := model.NewTaskRunModel(db)
	events := model.NewTaskEventModel(db)
	adapter := NewTaskEventQueryAdapter(events, runs)

	err := runs.Create(ctx, &model.TaskRun{
		TaskID:        "task-success",
		Status:        "succeeded",
		Progress:      100,
		Message:       "task finished",
		ResultPayload: datatypes.JSON([]byte(`{"ok":true}`)),
	})
	require.NoError(t, err)

	run, err := adapter.GetRun(ctx, "task-success")
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "task-success", run.GetTaskId())
	assert.Equal(t, "completed", run.GetType())
	assert.Equal(t, int32(100), run.GetProgress())
	assert.Equal(t, "task finished", run.GetMessage())
	assert.JSONEq(t, `{"ok":true}`, string(run.GetPayload()))
}

func TestTaskEventQueryAdapterGetRunUsesErrorMessageForFailedTask(t *testing.T) {
	t.Parallel()

	db := openDispatchTestDB(t)
	ctx := context.Background()
	runs := model.NewTaskRunModel(db)
	events := model.NewTaskEventModel(db)
	adapter := NewTaskEventQueryAdapter(events, runs)

	err := runs.Create(ctx, &model.TaskRun{
		TaskID:       "task-failed",
		Status:       "failed",
		Progress:     87,
		Message:      "stale message",
		ErrorMessage: "boom",
	})
	require.NoError(t, err)

	run, err := adapter.GetRun(ctx, "task-failed")
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "failed", run.GetType())
	assert.Equal(t, "boom", run.GetMessage())
	assert.Empty(t, run.GetPayload())
}

func TestDispatcherStreamTaskUsesPersistedRunState(t *testing.T) {
	t.Parallel()

	db := openDispatchTestDB(t)
	ctx := context.Background()
	runs := model.NewTaskRunModel(db)
	events := model.NewTaskEventModel(db)
	adapter := NewTaskEventQueryAdapter(events, runs)

	err := runs.Create(ctx, &model.TaskRun{
		TaskID:   "task-running",
		Status:   "running",
		Progress: 25,
		Message:  "working",
	})
	require.NoError(t, err)
	err = events.Append(ctx, &model.TaskEvent{
		TaskID:   "task-running",
		Seq:      1,
		Type:     "started",
		Progress: 25,
		Message:  "working",
	})
	require.NoError(t, err)

	dispatcher := NewDispatcherWithTaskStore(nil, nil, adapter)
	items, done, err := dispatcher.StreamTask(ctx, "task-running")
	require.NoError(t, err)
	assert.False(t, done)
	require.Len(t, items, 1)
	assert.Equal(t, "started", items[0].GetType())

	err = runs.UpdateByTaskID(ctx, "task-running", map[string]interface{}{
		"status":         "succeeded",
		"progress":       int32(100),
		"message":        "complete",
		"result_payload": datatypes.JSON([]byte(`{"answer":42}`)),
	})
	require.NoError(t, err)
	err = events.Append(ctx, &model.TaskEvent{
		TaskID:   "task-running",
		Seq:      2,
		Type:     "completed",
		Progress: 100,
		Message:  "complete",
		Payload:  datatypes.JSON([]byte(`{"answer":42}`)),
	})
	require.NoError(t, err)

	items, done, err = dispatcher.StreamTaskAfterSeq(ctx, "task-running", 1)
	require.NoError(t, err)
	assert.True(t, done)
	require.Len(t, items, 1)
	assert.Equal(t, "completed", items[0].GetType())
}

func openDispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}
