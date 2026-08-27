package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		ResultPayload: model.JSON([]byte(`{"ok":true}`)),
	})
	require.NoError(t, err)

	run, err := adapter.GetRun(ctx, "task-success")
	require.NoError(t, err)
	require.NotNil(t, run)
	assert.Equal(t, "task-success", run.TaskID)
	assert.Equal(t, "succeeded", run.Status)
	assert.Equal(t, int32(100), run.Progress)
	assert.Equal(t, "task finished", run.Message)
	assert.JSONEq(t, `{"ok":true}`, string(run.ResultPayload))
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
	assert.Equal(t, "failed", run.Status)
	assert.Equal(t, "boom", run.Message)
	assert.Empty(t, run.ResultPayload)
	assert.Equal(t, "boom", run.ErrorMessage)
}

func TestTaskEventQueryAdapterGetRunReturnsTypedNotFound(t *testing.T) {
	t.Parallel()

	db := openDispatchTestDB(t)
	ctx := context.Background()
	runs := model.NewTaskRunModel(db)
	events := model.NewTaskEventModel(db)
	adapter := NewTaskEventQueryAdapter(events, runs)

	run, err := adapter.GetRun(ctx, "missing-task")
	require.Nil(t, run)
	assert.ErrorIs(t, err, ErrTaskRunNotFound)
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
		"result_payload": model.JSON([]byte(`{"answer":42}`)),
	})
	require.NoError(t, err)
	err = events.Append(ctx, &model.TaskEvent{
		TaskID:   "task-running",
		Seq:      2,
		Type:     "completed",
		Progress: 100,
		Message:  "complete",
		Payload:  model.JSON([]byte(`{"answer":42}`)),
	})
	require.NoError(t, err)

	items, done, err = dispatcher.StreamTaskAfterSeq(ctx, "task-running", 1)
	require.NoError(t, err)
	assert.True(t, done)
	require.Len(t, items, 1)
	assert.Equal(t, "completed", items[0].GetType())
}

func TestDispatcherStreamTaskReturnsRunQueryErrors(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcherWithTaskStore(nil, nil, runErrorTaskQuery{})
	items, done, err := dispatcher.StreamTask(context.Background(), "task-error")
	require.Error(t, err)
	assert.Nil(t, items)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "query task run")
}

func TestDispatcherStreamTaskRealtimeAdvancesAfterSeq(t *testing.T) {
	db := openDispatchTestDB(t)
	ctx := context.Background()
	runs := model.NewTaskRunModel(db)
	events := model.NewTaskEventModel(db)
	adapter := NewTaskEventQueryAdapter(events, runs)

	err := runs.Create(ctx, &model.TaskRun{
		TaskID:   "task-realtime",
		Status:   "running",
		Progress: 10,
		Message:  "one",
	})
	require.NoError(t, err)
	err = events.Append(ctx, &model.TaskEvent{
		TaskID:   "task-realtime",
		Seq:      1,
		Type:     "progress",
		Progress: 10,
		Message:  "one",
	})
	require.NoError(t, err)

	oldPollInterval := taskStreamPollInterval
	taskStreamPollInterval = time.Millisecond
	defer func() {
		taskStreamPollInterval = oldPollInterval
	}()

	dispatcher := NewDispatcherWithTaskStore(nil, nil, adapter)
	streamCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var messages []string
	duplicate := false
	done, err := dispatcher.StreamTaskRealtime(streamCtx, "task-realtime", func(evt *sdkv1.TaskEvent) bool {
		message := evt.GetMessage()
		if message == "one" && len(messages) > 0 {
			duplicate = true
			return false
		}
		messages = append(messages, message)
		if message == "one" {
			require.NoError(t, runs.UpdateByTaskID(ctx, "task-realtime", map[string]interface{}{
				"status":   "succeeded",
				"progress": int32(100),
				"message":  "two",
			}))
			require.NoError(t, events.Append(ctx, &model.TaskEvent{
				TaskID:   "task-realtime",
				Seq:      2,
				Type:     "completed",
				Progress: 100,
				Message:  "two",
			}))
		}
		return true
	})

	require.NoError(t, err)
	assert.True(t, done)
	assert.False(t, duplicate)
	assert.Equal(t, []string{"one", "two"}, messages)
}

type runErrorTaskQuery struct{}

func (runErrorTaskQuery) ListEvents(context.Context, string, int64) ([]TaskEventRecord, error) {
	return []TaskEventRecord{
		{
			Seq: 1,
			Event: &sdkv1.TaskEvent{
				TaskId:  "task-error",
				Type:    "progress",
				Message: "event before run error",
			},
		},
	}, nil
}

func (runErrorTaskQuery) GetRun(context.Context, string) (*TaskRunState, error) {
	return nil, errors.New("database unavailable")
}

func openDispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gsqlite.Open("file::memory:?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}
