package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newExecLogDB(t *testing.T) *ExecutionLogModel {
	t.Helper()
	return NewExecutionLogModel(setupAllModelsDB(t))
}

func TestB2ExecutionLog_CreateAndGet(t *testing.T) {
	m := newExecLogDB(t)
	ctx := context.Background()

	log := &ExecutionLog{
		GameID:     "b2game",
		Env:        "prod",
		Source:     "invoke",
		FunctionID: "fn.list",
		Actor:      "admin",
		Status:     "ok",
	}
	require.NoError(t, m.Create(ctx, log))
	assert.NotZero(t, log.ID)

	got, err := m.Get(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, "b2game", got.GameID)
	assert.Equal(t, "fn.list", got.FunctionID)

	_, err = m.Get(ctx, 999999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestB2ExecutionLog_CreateError(t *testing.T) {
	m := NewExecutionLogModel(newClosedDB(t))
	assert.Error(t, m.Create(context.Background(), &ExecutionLog{GameID: "g"}))
}

func TestB2ExecutionLog_CreateBatch(t *testing.T) {
	m := newExecLogDB(t)
	ctx := context.Background()

	require.NoError(t, m.CreateBatch(ctx, nil))

	items := []ExecutionLog{
		{GameID: "b2game", Env: "prod", FunctionID: "f1", Actor: "a", Status: "ok"},
		{GameID: "b2game", Env: "prod", FunctionID: "f2", Actor: "a", Status: "error"},
	}
	require.NoError(t, m.CreateBatch(ctx, items))

	got, total, err := m.List(ctx, ExecutionLogListOptions{GameID: "b2game"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, got, 2)

	mErr := NewExecutionLogModel(newClosedDB(t))
	assert.Error(t, mErr.CreateBatch(ctx, []ExecutionLog{{GameID: "g"}}))
}

func TestB2ExecutionLog_ListFiltersAndPaging(t *testing.T) {
	m := newExecLogDB(t)
	ctx := context.Background()

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now().Add(24 * time.Hour)
	base := time.Now().UTC().Add(-time.Hour)
	rows := []ExecutionLog{
		{GameID: "b2game", Env: "prod", Source: "invoke", FunctionID: "fn.list", Actor: "op", Status: "ok", TraceID: "t1", CreatedAt: base},
		{GameID: "b2game", Env: "dev", Source: "page", FunctionID: "fn.get", Actor: "op", Status: "error", TraceID: "t2", CreatedAt: base},
		{GameID: "other", Env: "prod", Source: "invoke", FunctionID: "fn.list", Actor: "admin", Status: "ok", TraceID: "t3", CreatedAt: base},
	}
	for i := range rows {
		require.NoError(t, m.Create(ctx, &rows[i]))
	}

	cases := []ExecutionLogListOptions{
		{GameID: "b2game"},
		{Env: "dev"},
		{Actor: "op"},
		{FunctionID: "fn.list"},
		{Source: "page"},
		{Status: "error"},
		{TraceID: "t2"},
		{From: &from},
		{To: &to},
		{Page: 0, PageSize: 0},
		{PageSize: 500},
	}
	for _, opts := range cases {
		_, _, err := m.List(ctx, opts)
		assert.NoError(t, err, "opts=%+v", opts)
	}

	_, _, err := m.List(ctx, ExecutionLogListOptions{GameID: "b2game", Page: 2, PageSize: 1})
	assert.NoError(t, err)
}

func TestB2ExecutionLog_ListErrors(t *testing.T) {
	m := NewExecutionLogModel(newClosedDB(t))
	_, _, err := m.List(context.Background(), ExecutionLogListOptions{})
	assert.Error(t, err)

	db := setupAllModelsDB(t)
	require.NoError(t, db.Exec(
		`INSERT INTO execution_logs (game_id, env, source, function_id, actor, status, created_at)
		 VALUES ('g', 'prod', 'invoke', 'f', 'a', 'ok', 'not-a-time')`).Error)
	m2 := NewExecutionLogModel(db)
	_, _, err = m2.List(context.Background(), ExecutionLogListOptions{})
	assert.Error(t, err)
}

func TestB2ExecutionLog_DeleteBefore(t *testing.T) {
	m := newExecLogDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	cutoff := now.Add(-time.Minute)
	for i := 0; i < 5; i++ {
		require.NoError(t, m.Create(ctx, &ExecutionLog{
			GameID: "b2game", Env: "prod", FunctionID: "f", Actor: "a", Status: "ok", CreatedAt: now,
		}))
	}

	total, err := m.DeleteBefore(ctx, now.Add(time.Hour), 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)

	// batch<=0 defaults to 1000; second pass deletes nothing.
	total, err = m.DeleteBefore(ctx, cutoff, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)

	db := setupAllModelsDB(t)
	require.NoError(t, db.Migrator().DropTable(&ExecutionLog{}))
	_, err = NewExecutionLogModel(db).DeleteBefore(ctx, cutoff, 10)
	assert.Error(t, err)
}

func TestB2ExecutionLog_GetError(t *testing.T) {
	m := NewExecutionLogModel(newClosedDB(t))
	_, err := m.Get(context.Background(), 1)
	assert.Error(t, err)
}

func TestB2TaskModels_DeleteBefore(t *testing.T) {
	ctx := context.Background()

	runDB := setupAllModelsDB(t)
	tm := NewTaskRunModel(runDB)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		require.NoError(t, tm.Create(ctx, &TaskRun{TaskID: fmt.Sprintf("b2task-%d", i), GameID: "b2game", Status: "done"}))
	}
	n, err := tm.DeleteBefore(ctx, now.Add(time.Hour), 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	eventDB := setupAllModelsDB(t)
	em := NewTaskEventModel(eventDB)
	require.NoError(t, em.Append(ctx, &TaskEvent{TaskID: "b2task-0", Type: "log", CreatedAt: now}))
	n, err = em.DeleteBefore(ctx, now.Add(time.Hour), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
