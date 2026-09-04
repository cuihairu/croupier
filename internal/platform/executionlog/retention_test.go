package executionlog

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
)

func seedRetentionFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -30)
	recent := now.Add(-time.Hour)

	logs := []model.ExecutionLog{
		{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.old", Actor: "alice", Status: StatusOK, CreatedAt: old},
		{GameID: "g1", Env: "prod", Source: SourceInvoke, FunctionID: "f.new", Actor: "alice", Status: StatusOK, CreatedAt: recent},
	}
	require.NoError(t, db.Create(&logs).Error)

	runs := []model.TaskRun{
		{Model: gorm.Model{CreatedAt: old, UpdatedAt: old}, TaskID: "t-old", FunctionID: "job.old", GameID: "g1", Env: "prod", Status: "success"},
		{Model: gorm.Model{CreatedAt: recent, UpdatedAt: recent}, TaskID: "t-new", FunctionID: "job.new", GameID: "g1", Env: "prod", Status: "success"},
	}
	require.NoError(t, db.Create(&runs).Error)

	events := []model.TaskEvent{
		// TaskEvent 自带外层 CreatedAt（遮蔽 gorm.Model 的），必须直接赋值
		{TaskID: "t-old", Seq: 1, Type: "progress", CreatedAt: old},
		{TaskID: "t-new", Seq: 1, Type: "progress", CreatedAt: recent},
	}
	require.NoError(t, db.Create(&events).Error)
}

func TestRetentionSweepDeletesOnlyExpired(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 7, TaskLogDays: 7})
	summary := r.Sweep(context.Background())
	t.Logf("summary: exec=%d runs=%d events=%d execCutoff=%v taskCutoff=%v",
		summary.ExecutionLogsDeleted, summary.TaskRunsDeleted, summary.TaskEventsDeleted,
		summary.ExecutionLogCutoff, summary.TaskLogCutoff)

	assert.Equal(t, int64(1), summary.ExecutionLogsDeleted)
	assert.Equal(t, int64(1), summary.TaskRunsDeleted)
	assert.Equal(t, int64(1), summary.TaskEventsDeleted)

	var logCount, runCount, eventCount int64
	require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&logCount).Error)
	require.NoError(t, db.Model(&model.TaskRun{}).Count(&runCount).Error)
	require.NoError(t, db.Model(&model.TaskEvent{}).Count(&eventCount).Error)
	assert.Equal(t, int64(1), logCount)
	assert.Equal(t, int64(1), runCount)
	assert.Equal(t, int64(1), eventCount)

	// 剩下的是新记录
	var kept model.ExecutionLog
	require.NoError(t, db.First(&kept).Error)
	assert.Equal(t, "f.new", kept.FunctionID)
}

func TestRetentionZeroDaysKeepsEverything(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	// 0=永久保留
	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 0, TaskLogDays: 0})
	summary := r.Sweep(context.Background())

	assert.Equal(t, int64(0), summary.ExecutionLogsDeleted)
	assert.Equal(t, int64(0), summary.TaskRunsDeleted)
	assert.Equal(t, int64(0), summary.TaskEventsDeleted)

	var logCount, runCount int64
	require.NoError(t, db.Model(&model.ExecutionLog{}).Count(&logCount).Error)
	require.NoError(t, db.Model(&model.TaskRun{}).Count(&runCount).Error)
	assert.Equal(t, int64(2), logCount)
	assert.Equal(t, int64(2), runCount)
}

func TestConfigRetentionDefaults(t *testing.T) {
	// 未配置 → 默认 7 天
	var unset config.ExecutionLogConfig
	assert.Equal(t, 7, unset.EffectiveRetentionDays())
	assert.True(t, unset.IsEnabled())

	// 显式 0 → 永久
	zero := 0
	explicit := config.ExecutionLogConfig{RetentionDays: &zero}
	assert.Equal(t, 0, explicit.EffectiveRetentionDays())

	seven := 7
	set := config.TaskLogConfig{RetentionDays: &seven}
	assert.Equal(t, 7, set.EffectiveRetentionDays())
}
