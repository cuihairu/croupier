package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newRunbookDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Runbook{}, &RunbookRun{}))
	return db
}

// TestRunbookModel_CRUD 建/查/列/删 + scope 隔离。
func TestRunbookModel_CRUD(t *testing.T) {
	db := newRunbookDB(t)
	m := NewRunbookModel(db)
	ctx := context.Background()

	rb := &Runbook{RunbookID: "rb-1", Name: "rollout", GameID: "g1", Env: "prod", StepsJSON: `[{"name":"s"}]`, CreatedBy: "admin"}
	require.NoError(t, m.Create(ctx, rb))

	got, err := m.Find(ctx, "g1", "prod", "rb-1")
	require.NoError(t, err)
	assert.Equal(t, "rollout", got.Name)

	// 跨 scope 查不到
	_, err = m.Find(ctx, "g2", "prod", "rb-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	// 列表按 scope 过滤
	_ = m.Create(ctx, &Runbook{RunbookID: "rb-2", Name: "b", GameID: "g1", Env: "staging", StepsJSON: `[]`})
	items, err := m.List(ctx, "g1", "prod", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "rb-1", items[0].RunbookID)

	// 删除 + 不存在报错
	require.NoError(t, m.Delete(ctx, "g1", "prod", "rb-1"))
	_, err = m.Find(ctx, "g1", "prod", "rb-1")
	require.Error(t, err)
	require.Error(t, m.Delete(ctx, "g1", "prod", "rb-1"))
}

// TestRunbookRunModel_Lifecycle 运行记录创建/查/列/终态回写。
func TestRunbookRunModel_Lifecycle(t *testing.T) {
	db := newRunbookDB(t)
	m := NewRunbookRunModel(db)
	ctx := context.Background()

	run := &RunbookRun{RunID: "run-1", RunbookID: "rb-1", GameID: "g1", Env: "prod", Status: RunbookRunStatusRunning, StartedAt: time.Now().UTC()}
	require.NoError(t, m.Create(ctx, run))

	got, err := m.Find(ctx, "g1", "prod", "run-1")
	require.NoError(t, err)
	assert.Equal(t, RunbookRunStatusRunning, got.Status)

	// 终态回写：status/currentStep/stepsResult/error/finishedAt
	require.NoError(t, m.UpdateStatus(ctx, run.ID, RunbookRunStatusSucceeded, 3, `[{"status":"success"}]`, ""))
	got, err = m.Find(ctx, "g1", "prod", "run-1")
	require.NoError(t, err)
	assert.Equal(t, RunbookRunStatusSucceeded, got.Status)
	assert.Equal(t, 3, got.CurrentStep)
	assert.Equal(t, `[{"status":"success"}]`, got.StepsResultJSON)
	require.NotNil(t, got.FinishedAt)

	// 按 runbook 列出
	runs, err := m.ListByRunbook(ctx, "g1", "prod", "rb-1", 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)

	// scope 隔离
	_, err = m.Find(ctx, "g2", "prod", "run-1")
	require.Error(t, err)
}
