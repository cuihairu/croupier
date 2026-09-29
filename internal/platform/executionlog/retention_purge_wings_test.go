package executionlog

// 覆盖率巡检第十六轮（wt-api）：retention.go PurgeBefore 残余 2 块收口——
// execution 段存储错误直传翼（execution_logs 缺表）与 task 段错误翼
// （task_events 缺表：runs 已删、events 报错 → 部分摘要随错误上抛）。

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

func TestRetentionPurgeBefore_ErrorWings(t *testing.T) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	// execution 段失败：缺表直传
	db := newTestDB(t)
	seedRetentionFixtures(t, db)
	require.NoError(t, db.Migrator().DropTable(&model.ExecutionLog{}))
	_, err := NewRetention(db, RetentionConfig{}).PurgeBefore(context.Background(), cutoff, "execution")
	require.Error(t, err, "execution_logs 缺表应报错")

	// task 段失败：task_events 缺表（task_runs 先删成功）→ 错误仍上抛
	db2 := newTestDB(t)
	seedRetentionFixtures(t, db2)
	require.NoError(t, db2.Migrator().DropTable(&model.TaskEvent{}))
	_, err = NewRetention(db2, RetentionConfig{}).PurgeBefore(context.Background(), cutoff, "all")
	require.Error(t, err, "task_events 缺表应报错")
}
