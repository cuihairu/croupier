package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestTaskRunModel_CRUD(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&TaskRun{}, &TaskEvent{})
	require.NoError(t, err)

	runModel := NewTaskRunModel(db)
	_ = NewTaskEventModel(db)
	ctx := context.Background()

	// Create task run
	task := &TaskRun{
		TaskID:     "task-001",
		FunctionID: "player.ban",
		GameID:     "game1",
		Env:        "prod",
		Status:     "running",
	}
	err = runModel.Create(ctx, task)
	require.NoError(t, err)

	// Find by task ID
	found, err := runModel.FindByTaskID(ctx, "task-001")
	require.NoError(t, err)
	assert.Equal(t, "player.ban", found.FunctionID)

	// Find by task ID not found
	_, err = runModel.FindByTaskID(ctx, "nonexistent")
	assert.Error(t, err)

	// Find by empty task ID
	_, err = runModel.FindByTaskID(ctx, "")
	assert.Error(t, err)

	// Update by task ID
	err = runModel.UpdateByTaskID(ctx, "task-001", map[string]interface{}{
		"status": "succeeded",
	})
	require.NoError(t, err)

	// Verify update
	found, err = runModel.FindByTaskID(ctx, "task-001")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", found.Status)

	// A late lifecycle event must not undo a terminal task state.
	updated, err := runModel.UpdateByTaskIDIfStatusNotIn(ctx, "task-001", []string{"succeeded", "failed", "cancelled", "timed_out"}, map[string]interface{}{
		"status": "cancel_requested",
	})
	require.NoError(t, err)
	assert.False(t, updated)
	found, err = runModel.FindByTaskID(ctx, "task-001")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", found.Status)

	// Update by empty task ID
	err = runModel.UpdateByTaskID(ctx, "", map[string]interface{}{"status": "failed"})
	assert.Error(t, err)

	// List tasks
	tasks, total, err := runModel.List(ctx, ListTasksOptions{})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, int64(1), total)

	// List with filters
	tasks, _, err = runModel.List(ctx, ListTasksOptions{FunctionID: "player.ban"})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	tasks, _, err = runModel.List(ctx, ListTasksOptions{Status: "succeeded"})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	tasks, _, err = runModel.List(ctx, ListTasksOptions{GameID: "game1", Env: "prod"})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	// List with no results
	tasks, total, err = runModel.List(ctx, ListTasksOptions{FunctionID: "nonexistent"})
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
	assert.Equal(t, int64(0), total)
}

// ===== TaskEventModel Tests =====

func TestTaskEventModel_AppendAndList(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&TaskEvent{})
	require.NoError(t, err)

	model := NewTaskEventModel(db)
	ctx := context.Background()

	// Append events
	event1 := &TaskEvent{
		TaskID:    "task-evt-001",
		Seq:       1,
		Type:      "started",
		Progress:  0,
		Message:   "Task started",
		Payload:   []byte(`{}`),
		CreatedAt: time.Now(),
	}
	err = model.Append(ctx, event1)
	require.NoError(t, err)

	event2 := &TaskEvent{
		TaskID:    "task-evt-001",
		Seq:       2,
		Type:      "progress",
		Progress:  50,
		Message:   "Halfway",
		Payload:   []byte(`{}`),
		CreatedAt: time.Now(),
	}
	err = model.Append(ctx, event2)
	require.NoError(t, err)

	// List events
	events, err := model.ListByTaskID(ctx, "task-evt-001", 0)
	require.NoError(t, err)
	assert.Len(t, events, 2)

	// List with afterSeq
	events, err = model.ListByTaskID(ctx, "task-evt-001", 1)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// NextSeq
	seq, err := model.NextSeq(ctx, "task-evt-001")
	require.NoError(t, err)
	assert.Equal(t, int64(3), seq)

	// NextSeq for new task
	seq, err = model.NextSeq(ctx, "task-evt-002")
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)

	// ListByTaskID with empty task ID
	events, err = model.ListByTaskID(ctx, "", 0)
	require.NoError(t, err)
	assert.Len(t, events, 0)

	// NextSeq with empty task ID
	seq, err = model.NextSeq(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)
}

// ===== EncodeTaskPayload Tests =====

func TestEncodeTaskPayload(t *testing.T) {
	// nil
	result := EncodeTaskPayload(nil)
	assert.Equal(t, "null", string(result))

	// []byte
	result = EncodeTaskPayload([]byte(`{"key":"value"}`))
	assert.Equal(t, `{"key":"value"}`, string(result))

	// string
	result = EncodeTaskPayload(`{"key":"value"}`)
	assert.Equal(t, `{"key":"value"}`, string(result))

	// struct
	type TestStruct struct {
		Name string `json:"name"`
	}
	result = EncodeTaskPayload(TestStruct{Name: "test"})
	assert.Contains(t, string(result), "test")

	// map
	result = EncodeTaskPayload(map[string]int{"a": 1})
	assert.Contains(t, string(result), "1")
}
