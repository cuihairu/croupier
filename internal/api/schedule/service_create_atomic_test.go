// AUD-088 regression coverage: a new active schedule and its first
// next_triggered_at must land in ONE insert. The previous shape inserted the row
// first and then back-filled next_triggered_at with a separate UPDATE whose
// failure was swallowed -- ListDue only scans next_triggered_at IS NOT NULL, so
// such a row is a schedule that can never fire while the API reported success.
package schedule

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func countSchedWrites(t *testing.T, db *gorm.DB) (inserts *atomic.Int64, updates *atomic.Int64) {
	t.Helper()
	inserts, updates = &atomic.Int64{}, &atomic.Int64{}
	require.NoError(t, db.Callback().Create().After("gorm:create").Register("aud088_count_insert", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_schedules" {
			inserts.Add(1)
		}
	}))
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("aud088_count_update", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_schedules" {
			updates.Add(1)
		}
	}))
	return inserts, updates
}

// TestCreate_FirstNextSameInsert proves the atomic write: creating an active
// schedule performs exactly one INSERT on task_schedules and zero UPDATEs, and
// the row already carries next_triggered_at when the request succeeds.
func TestCreate_FirstNextSameInsert(t *testing.T) {
	r, db := newTestEnv(t, true)
	inserts, updates := countSchedWrites(t, db)

	start := time.Now().UTC()
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", createBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, int64(1), inserts.Load(), "创建调度必须只发生一次 INSERT")
	assert.Equal(t, int64(0), updates.Load(), "创建不得依赖第二次 UPDATE 回填首触时间（AUD-088）")

	var resp CreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Greater(t, resp.Item.ID, uint(0))
	assert.NotEmpty(t, resp.Item.NextTriggerAt, "响应必须携带首触时间")

	var row model.TaskSchedule
	require.NoError(t, db.First(&row, resp.Item.ID).Error)
	require.NotNil(t, row.NextTriggeredAt, "首触时间必须随 INSERT 落库（ListDue 扫描条件）")
	assert.Equal(t, model.ScheduleStatusActive, row.Status)
	assert.False(t, row.NextTriggeredAt.Before(start), "首触时间必须晚于创建时刻")

	respNext, err := time.Parse(time.RFC3339, resp.Item.NextTriggerAt)
	require.NoError(t, err)
	assert.True(t, respNext.Equal(row.NextTriggeredAt.UTC()), "响应首触时间必须与落库值一致")
}

// TestCreate_InsertFailureReturnsErrorNoRow: the insert is the only write, so a
// storage failure surfaces as a contract error (500 + {error,message}) and
// leaves no schedule row behind.
func TestCreate_InsertFailureReturnsErrorNoRow(t *testing.T) {
	r, db := newTestEnv(t, true)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("aud088_fail_insert", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_schedules" {
			_ = tx.AddError(errSchedFailpoint)
		}
	}))

	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", createBody)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "internal_error")

	var total int64
	require.NoError(t, db.Model(&model.TaskSchedule{}).Count(&total).Error)
	assert.Equal(t, int64(0), total, "落库失败不得留下半截调度行")
}
