// Coverage target: the m.SetStatus failure branch inside
// schedule service.SetStatus(active) (service.go:188-190).
//
// SetStatus(active) runs two UPDATEs in sequence: it recomputes next_triggered_at
// first, then lands status. The counting baseline of the existing case (the
// active_second_update_fails case in schedule_extra_test.go) includes the UPDATE
// performed at creation time, so it actually hits the recomputation; this file
// registers the "fail from the 2nd time on" callback only after the schedule has
// been created, so the recomputation succeeds and the status update fails.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestService_SetStatus_ActiveStatusUpdateFailsGroupE(t *testing.T) {
	name := fmt.Sprintf("schedule_groupE_%d", atomic.AddUint64(&dbSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	m := model.NewTaskScheduleModel(db)
	s := NewService(&svc.ServiceContext{TaskScheduleModel: m})
	ctx := context.WithValue(context.Background(), "username", "tester")

	created, err := m.Create(ctx, model.CreateScheduleInput{
		Name: "groupE", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod",
		FunctionID: "fn.reset", Actor: "tester",
	})
	require.NoError(t, err)

	var calls int32
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("groupE_fail_second_update", func(tx *gorm.DB) {
			if atomic.AddInt32(&calls, 1) >= 2 {
				_ = tx.AddError(errors.New("status update boom"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("groupE_fail_second_update") })

	item, err := s.SetStatus(ctx, created.ID, model.ScheduleStatusActive)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status update boom")
	assert.Equal(t, ScheduleItem{}, item)
}
