// 覆盖目标：sweepExecutionLogs 的 meta 库删除失败、sweepTaskLogs 的
// task_runs 删除失败分支。
package executionlog

import (
	"context"
	"errors"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func TestV8RetentionSweep_ExecutionLogDeleteError(t *testing.T) {
	db := newTestDB(t)
	seedRetentionFixtures(t, db)

	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("v8:delfail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "execution_logs" {
			_ = tx.AddError(errors.New("injected delete failure"))
		}
	}))

	r := NewRetention(db, RetentionConfig{ExecutionLogDays: 7, TaskLogDays: 7})
	summary := r.Sweep(context.Background())
	assert.Equal(t, int64(0), summary.ExecutionLogsDeleted)
}

func TestV8RetentionSweep_TaskRunDeleteError(t *testing.T) {
	// 只迁移 task_events：task_runs 清理失败 → 返回 0/0 与错误。
	bare, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, bare.AutoMigrate(&model.TaskEvent{}))

	r := NewRetention(bare, RetentionConfig{TaskLogDays: 7})
	summary := r.Sweep(context.Background())
	assert.Equal(t, int64(0), summary.TaskRunsDeleted)
	assert.Equal(t, int64(0), summary.TaskEventsDeleted)
}
