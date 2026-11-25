// Coverage target: the error branches of the schedule service (uninitialized model,
// invalid status, missing scope, storage error, invalid cron) and the service
// error paths of the handler.
package schedule

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestService_Methods_ModelNotInitialized(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	ctx := context.Background()

	_, err := s.List(ctx, &ListRequest{})
	require.Error(t, err)
	_, err = s.Create(ctx, &CreateRequest{Name: "n", CronExpr: "* * * * *", FunctionID: "f"})
	require.Error(t, err)
	_, err = s.SetStatus(ctx, 1, model.ScheduleStatusActive)
	require.Error(t, err)
	_, err = s.TriggerNow(ctx, 1)
	require.Error(t, err)
	_, err = s.RunLogs(ctx, 1, 1, 10)
	require.Error(t, err)
	require.Error(t, s.Delete(ctx, 1))
}

func TestService_Create_InvalidCron(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", `{"name":"bad","cronExpr":"not-a-cron","functionId":"f","gameId":"demo","env":"prod"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestService_Create_MissingScope(t *testing.T) {
	// No GameScope injected and the request body carries no gameId/env
	r, _ := newTestEnv(t, false)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", `{"name":"n","cronExpr":"* * * * *","functionId":"f"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestService_SetStatus_InvalidValue(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)

	rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/status", `{"status":"bogus"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestService_SetStatus_PausedThenActive(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)
	target := "/api/v1/schedules/" + strconv.FormatUint(uint64(id), 10) + "/status"

	rec := doJSON(r, http.MethodPut, target, `{"status":"paused"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "paused")

	rec2 := doJSON(r, http.MethodPut, target, `{"status":"active"}`)
	assert.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	assert.Contains(t, rec2.Body.String(), "active")
}

func TestService_SetStatus_UnknownSchedule(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doJSON(r, http.MethodPut, "/api/v1/schedules/99999/status", `{"status":"active"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestService_TriggerNow_UnknownSchedule(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/99999/trigger")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestService_RunLogs_AfterCreate_Empty(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)

	rec := doReq(r, http.MethodGet, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/runs")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"total":0`)
}

func TestService_Delete_Unknown_Idempotent(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodDelete, "/api/v1/schedules/99999")
	// Ownership must be checked before deleting; an unknown id returns 404, keeping
	// cross-scope resources from being discoverable.
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestService_List_StoreError(t *testing.T) {
	r, db := newTestEnv(t, true)
	require.NoError(t, db.Migrator().DropTable("task_schedules"))
	rec := doReq(r, http.MethodGet, "/api/v1/schedules")
	assert.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

func TestService_List_StatusFilter_NoMatch(t *testing.T) {
	r, _ := newTestEnv(t, true)
	mustCreate(t, r)
	rec := doReq(r, http.MethodGet, "/api/v1/schedules?status=paused")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"total":0`)
}

func TestHandler_List_InvalidPageQuery(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodGet, "/api/v1/schedules?page=abc")
	// After the Bug4 fix: a numeric query conversion error returns 400 per the contract
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_RunLogs_InvalidID(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodGet, "/api/v1/schedules/not-a-number/runs")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_SetStatus_MissingBody(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)
	rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/status", ``)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHandler_Delete_InvalidID(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodDelete, "/api/v1/schedules/not-a-number")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestService_TriggerNow_DispatchFails(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)
	// fn.reset has no registered agent: dispatch should fail
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/trigger")
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestService_PauseClearsNextTrigger(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	target := "/api/v1/schedules/" + strconv.FormatUint(uint64(id), 10) + "/status"

	// A created schedule carries nextTriggerAt by default; pausing clears it -> the nil
	// branch of buildItem
	rec := doJSON(r, http.MethodPut, target, `{"status":"paused"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "nextTriggerAt")

	var sch model.TaskSchedule
	require.NoError(t, db.Where("id = ?", id).First(&sch).Error)
	assert.Nil(t, sch.NextTriggeredAt)
}

func TestService_SetStatus_UpdateFailures(t *testing.T) {
	newEnvWithUpdateFailure := func(t *testing.T, failFrom int32) (*gin.Engine, *gorm.DB) {
		r, db := newTestEnv(t, true)
		var calls int32
		require.NoError(t, db.Callback().Update().Before("gorm:update").
			Register("test:sched_update_fail", func(tx *gorm.DB) {
				if atomic.AddInt32(&calls, 1) >= failFrom {
					_ = tx.AddError(errors.New("update boom"))
				}
			}))
		t.Cleanup(func() { _ = db.Callback().Update().Remove("test:sched_update_fail") })
		return r, db
	}
	id := func(t *testing.T, r *gin.Engine) string {
		return strconv.FormatUint(uint64(mustCreate(t, r)), 10)
	}

	t.Run("active_first_update_fails", func(t *testing.T) {
		r, _ := newEnvWithUpdateFailure(t, 1)
		rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+id(t, r)+"/status", `{"status":"active"}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("active_second_update_fails", func(t *testing.T) {
		r, _ := newEnvWithUpdateFailure(t, 2)
		rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+id(t, r)+"/status", `{"status":"active"}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("paused_update_fails", func(t *testing.T) {
		r, _ := newEnvWithUpdateFailure(t, 1)
		rec := doJSON(r, http.MethodPut, "/api/v1/schedules/"+id(t, r)+"/status", `{"status":"paused"}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandler_TriggerNow_StoreError(t *testing.T) {
	r, db := newTestEnv(t, true)
	scheduleID := mustCreate(t, r)
	require.NoError(t, db.Migrator().DropTable("task_schedules"))
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+strconv.FormatUint(uint64(scheduleID), 10)+"/trigger")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestService_TriggerNow_DispatchError(t *testing.T) {
	r, db := newTestEnv(t, true)
	scheduleID := mustCreate(t, r)
	// Blank the schedules table to break function routing before FindByID hits:
	// zeroing out the dispatcher registry directly is not feasible, so instead make
	// the function ID unroutable (dispatch errors internally). Approach: set the
	// function ID to a nonexistent value; the Dispatcher cannot find the function
	// and errors.
	require.NoError(t, db.Exec("UPDATE task_schedules SET function_id = 'missing.fn' WHERE id = ?", scheduleID).Error)
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+strconv.FormatUint(uint64(scheduleID), 10)+"/trigger")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}
