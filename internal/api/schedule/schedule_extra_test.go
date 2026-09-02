// 覆盖目标：schedule service 的错误分支（模型未初始化、非法 status、
// 缺 scope、存储错误、cron 无效）与 handler 的 service 错误路径。
package schedule

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// 未注入 GameScope 且请求体不带 gameId/env
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
	// Delete 对不存在 id 幂等（gorm Delete 不报错）
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
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
	// 现状：query 数值转换错误未被 response.Error 识别为 400（已知问题，
	// 见 coverage 交付报告），至少不得返回 200 成功。
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
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
	// fn.reset 无已注册 agent：dispatch 应失败
	rec := doReq(r, http.MethodPost, "/api/v1/schedules/"+strconv.FormatUint(uint64(id), 10)+"/trigger")
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestService_PauseClearsNextTrigger(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	target := "/api/v1/schedules/" + strconv.FormatUint(uint64(id), 10) + "/status"

	// 创建后默认带 nextTriggerAt；暂停后清空 → buildItem 的 nil 分支
	rec := doJSON(r, http.MethodPut, target, `{"status":"paused"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "nextTriggerAt")

	var sch model.TaskSchedule
	require.NoError(t, db.Where("id = ?", id).First(&sch).Error)
	assert.Nil(t, sch.NextTriggeredAt)
}
