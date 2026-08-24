package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskEventQueryAdapter_EdgeCases(t *testing.T) {
	adapter := NewTaskEventQueryAdapter(nil, nil)

	records, err := adapter.ListEvents(context.Background(), "   ", 0)
	require.NoError(t, err)
	assert.Empty(t, records)

	run, err := adapter.GetRun(context.Background(), "")
	assert.Nil(t, run)
	assert.ErrorIs(t, err, ErrTaskRunNotFound)

	assert.Equal(t, "", taskRunMessage(nil))
}

func TestTaskRunWriterAdapter_NilModelIsNoop(t *testing.T) {
	writer := NewTaskRunWriterAdapter(nil)
	require.NoError(t, writer.CreateRun(context.Background(), "t", "fn", "agent", "game", "dev", "dispatching", []byte("{}")))
	require.NoError(t, writer.CreateRunWithMeta(context.Background(), "t", "fn", "agent", "game", "dev", "dispatching", "actor", "addr", "trace", []byte("{}")))
}

func TestTaskRunWriterAdapter_PersistsRuns(t *testing.T) {
	db := openDispatchTestDB(t)
	writer := NewTaskRunWriterAdapter(model.NewTaskRunModel(db))

	require.NoError(t, writer.CreateRun(context.Background(), "run-plain", "fn.a", "agent-1", "game-1", "dev", "dispatching", []byte(`{"in":1}`)))

	require.NoError(t, writer.CreateRunWithMeta(context.Background(), "run-meta", "fn.b", "agent-2", "game-1", "prod", "dispatching", "admin", "10.0.0.1:9090", "trace-123", []byte(`{"in":2}`)))
	require.NoError(t, writer.CreateRunWithMeta(context.Background(), "run-no-trace", "fn.c", "agent-3", "game-1", "prod", "dispatching", "admin", "", "", []byte(`{}`)))

	runs := model.NewTaskRunModel(db)
	ctx := context.Background()

	run, err := runs.FindByTaskID(ctx, "run-plain")
	require.NoError(t, err)
	assert.Equal(t, "fn.a", run.FunctionID)
	assert.Equal(t, "agent-1", run.AgentID)
	assert.Equal(t, "game-1", run.GameID)
	assert.Equal(t, "dev", run.Env)

	run, err = runs.FindByTaskID(ctx, "run-meta")
	require.NoError(t, err)
	assert.Equal(t, "admin", run.Actor)
	assert.Equal(t, "10.0.0.1:9090", run.Addr)
	assert.Equal(t, "trace-123", run.TraceID)

	run, err = runs.FindByTaskID(ctx, "run-no-trace")
	require.NoError(t, err)
	assert.Empty(t, run.TraceID)
}

func TestNewFileTaskRoutingStore_MkdirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("file"), 0644))

	_, err := NewFileTaskRoutingStore(blocker)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create data directory")
}

func TestNewFileTaskRoutingStore_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task_routing.json"), []byte("not-json"), 0644))

	_, err := NewFileTaskRoutingStore(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load task routing data")
}

func TestNewFileTaskRoutingStore_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task_routing.json"), nil, 0644))

	store, err := NewFileTaskRoutingStore(dir)
	require.NoError(t, err)
	list, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestFileTaskRoutingStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dir)
	require.NoError(t, err)

	require.NoError(t, store.Set("task-1", "agent-1"))
	require.NoError(t, store.Set("task-2", "agent-2"))
	// Re-setting keeps the original creation timestamp.
	first, err := store.Get("task-1")
	require.NoError(t, err)
	createdAt := first.CreatedAt
	require.NoError(t, store.Set("task-1", "agent-1b"))

	reloaded, err := NewFileTaskRoutingStore(dir)
	require.NoError(t, err)
	routing, err := reloaded.Get("task-1")
	require.NoError(t, err)
	assert.Equal(t, "agent-1b", routing.AgentID)
	assert.Equal(t, createdAt.UTC(), routing.CreatedAt.UTC())

	require.NoError(t, reloaded.Delete("task-2"))
	_, err = reloaded.Get("task-2")
	assert.Error(t, err)
}

func TestFileTaskRoutingStore_GetMissing(t *testing.T) {
	store, err := NewFileTaskRoutingStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Get("missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task routing not found")
}

func TestFileTaskRoutingStore_SaveFailure(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dir)
	require.NoError(t, err)

	// Replacing the persistence file location with a directory makes the
	// atomic rename fail.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "task_routing.json"), 0755))

	err = store.Set("task-fail", "agent-1")
	require.Error(t, err)
}

func TestFileTaskRoutingStore_CleanupWithoutDeletesSkipsSave(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileTaskRoutingStore(dir)
	require.NoError(t, err)
	require.NoError(t, store.Set("task-fresh", "agent-1"))

	require.NoError(t, store.Cleanup(time.Hour))
	routing, err := store.Get("task-fresh")
	require.NoError(t, err)
	assert.Equal(t, "agent-1", routing.AgentID)
}
