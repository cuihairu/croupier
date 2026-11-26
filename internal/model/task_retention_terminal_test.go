package model

// AUD-058：任务保留期以终态为清理边界——运行、排队、等待确认（finished_at
// IS NULL）的任务与事件超过创建时间阈值仍可继续查询；已结束任务以确定的
// 终态时间（finished_at，所有终态写入路径都会盖）判断保留期；关联事件与
// 所属任务同生命周期。

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTaskRetentionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TaskRun{}, &TaskEvent{}))
	return db
}

// TestTaskRunDeleteBefore_TerminalBoundary 终态边界：只删终态时间超龄的行；
// 运行/排队/等待确认的超龄任务保留；终态未超龄的保留。
func TestTaskRunDeleteBefore_TerminalBoundary(t *testing.T) {
	db := newTaskRetentionDB(t)
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	recent := now.Add(-time.Hour)
	rows := []TaskRun{
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-finished-old", Status: "succeeded", FinishedAt: &old},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-running-old", Status: "running"},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-queued-old", Status: "queued"},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-cancelreq-old", Status: "cancel_requested"},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: recent}, TaskID: "t-finished-recent", Status: "succeeded", FinishedAt: &recent},
	}
	require.NoError(t, db.Create(&rows).Error)

	deleted, err := NewTaskRunModel(db).DeleteBefore(context.Background(), now.AddDate(0, 0, -7), 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var ids []string
	require.NoError(t, db.Model(&TaskRun{}).Order("task_id").Pluck("task_id", &ids).Error)
	assert.Equal(t, []string{"t-cancelreq-old", "t-finished-recent", "t-queued-old", "t-running-old"}, ids)
}

// TestTaskRunDeleteBefore_ConcurrentFinishMidRetention 状态并发变化：超龄
// 运行中任务在扫描期间到达终态（finished_at=now），下轮清理以终态时间
// 判断——终态未超龄不误删。
func TestTaskRunDeleteBefore_ConcurrentFinishMidRetention(t *testing.T) {
	db := newTaskRetentionDB(t)
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	run := TaskRun{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-late-finish", Status: "running"}
	require.NoError(t, db.Create(&run).Error)
	cutoff := now.AddDate(0, 0, -7)

	// 第一轮：仍在运行（finished_at NULL）→ 保留
	deleted, err := NewTaskRunModel(db).DeleteBefore(context.Background(), cutoff, 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	// 并发到达终态：finished_at=now
	require.NoError(t, db.Model(&TaskRun{}).Where("task_id = ?", "t-late-finish").
		Update("finished_at", now).Error)

	// 第二轮：终态时间 now 未超龄 → 仍保留
	deleted, err = NewTaskRunModel(db).DeleteBefore(context.Background(), cutoff, 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

// TestTaskEventDeleteBefore_FollowsTaskLifecycle 关联事件清理：事件随所属
// 任务同生命周期——未结束任务的事件保留；已结束且终态超龄任务的事件清理
// （含任务行已删的孤儿事件）；已结束但终态未超龄任务的事件保留。
func TestTaskEventDeleteBefore_FollowsTaskLifecycle(t *testing.T) {
	db := newTaskRetentionDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	recent := now.Add(-time.Hour)
	cutoff := now.AddDate(0, 0, -7)

	runs := []TaskRun{
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-done-expired", Status: "succeeded", FinishedAt: &old},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-still-running", Status: "running"},
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: recent}, TaskID: "t-done-recent", Status: "succeeded", FinishedAt: &recent},
	}
	require.NoError(t, db.Create(&runs).Error)
	events := []TaskEvent{
		{TaskID: "t-done-expired", Seq: 1, Type: "progress", CreatedAt: old},
		{TaskID: "t-still-running", Seq: 1, Type: "progress", CreatedAt: old},
		{TaskID: "t-done-recent", Seq: 1, Type: "progress", CreatedAt: old},
		{TaskID: "t-orphan", Seq: 1, Type: "progress", CreatedAt: old}, // 任务行已删
	}
	require.NoError(t, db.Create(&events).Error)

	// 与 retention 同序：先清 run 再清 event（本轮删掉的 run 的事件按孤儿清）
	runDeleted, err := NewTaskRunModel(db).DeleteBefore(ctx, cutoff, 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(1), runDeleted) // t-done-expired
	eventDeleted, err := NewTaskEventModel(db).DeleteBefore(ctx, cutoff, 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(2), eventDeleted) // t-done-expired 的事件 + t-orphan 孤儿

	var ids []string
	require.NoError(t, db.Model(&TaskEvent{}).Order("task_id").Pluck("task_id", &ids).Error)
	// 运行中任务与终态未超龄任务的事件保留（超龄也留，可继续查询）
	assert.Equal(t, []string{"t-done-recent", "t-still-running"}, ids)
}
