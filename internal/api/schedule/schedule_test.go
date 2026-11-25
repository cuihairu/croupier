package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var dbSeq uint64

// newTestEnv assembles a route-level test environment: in-memory sqlite plus a
// dispatcher with an empty registry.
// scoped controls whether a GameScope is injected (simulating GameDBMiddleware).
func newTestEnv(t *testing.T, scoped bool) (*gin.Engine, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("schedule_%d", atomic.AddUint64(&dbSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		AdminModel:        model.NewAdminModel(db),
		// ARCH-024 (R30-01): Govern now runs the explicit function ACL in
		// CheckInvokePermission, and a nil FunctionModel fails closed (403). Inject a
		// FunctionModel as the task service tests do so TriggerNow can reach the
		// dispatch path.
		FunctionModel: model.NewFunctionModel(db),
		Dispatcher:    dispatch.NewDispatcher(reg.NewStore()),
	}
	// TriggerNow loads the caller principal through the ARCH-024 Govern chain
	// (username=tester), so a matching Admin row must exist; the other entry points
	// never touch principal loading, so this seed only affects the trigger path.
	seedScheduleAdmin(t, db, "tester")
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "tester")
		if scoped {
			ctx = svc.WithGameScope(ctx, svc.GameScope{GameID: "demo", Env: "prod"})
		}
		c.Request = c.Request.WithContext(ctx)
	})
	g := r.Group("/api/v1/schedules")
	g.GET("", h.List)
	g.GET("/", h.List)
	g.POST("", h.Create)
	g.POST("/", h.Create)
	g.GET("/:id/runs", h.RunLogs)
	g.PUT("/:id/status", h.SetStatus)
	g.POST("/:id/trigger", h.TriggerNow)
	g.DELETE("/:id", h.Delete)
	return r, db
}

func doJSON(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// seedScheduleAdmin prepares the caller Admin row for TriggerNow's Govern
// principal loading and binds the admin role (HasAdminRole bypasses
// CheckInvokePermission) -- since ARCH-024 Govern runs the explicit function ACL,
// and an admin with no role is denied fail-closed, making the dispatch path
// untestable. When PolicyManager is nil the whole policy chain is skipped and the
// role no longer matters.
func seedScheduleAdmin(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	role := &model.Role{Name: "admin", Category: "test"}
	require.NoError(t, db.Create(role).Error)
	admin := &model.Admin{Username: username, Nickname: username, Status: 1}
	require.NoError(t, db.Create(admin).Error)
	require.NoError(t, db.Create(&model.AdminRole{AdminID: admin.ID, RoleID: role.ID}).Error)
}

func doReq(r *gin.Engine, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

const createBody = `{"name":"每日重置","cronExpr":"*/5 * * * *","functionId":"fn.reset","gameId":"demo","env":"prod","payload":{"k":1},"maxFailedRuns":3}`

func mustCreate(t *testing.T, r *gin.Engine) uint {
	t.Helper()
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", createBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp CreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Greater(t, resp.Item.ID, uint(0))
	return resp.Item.ID
}

func TestList_Empty(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doReq(r, http.MethodGet, "/api/v1/schedules")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"items":[],"total":0}`, rec.Body.String())
}

func TestCreate_List_Filters(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)

	// Creating a cross-game schedule succeeds, but it is invisible to a scoped request
	// (the ctx scope takes precedence over the query params).
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules",
		`{"name":"跨游戏","cronExpr":"0 3 * * *","functionId":"fn.other","gameId":"other","env":"prod"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var list ListResponse
	rec = doReq(r, http.MethodGet, "/api/v1/schedules")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(1), list.Total)

	// Add another demo schedule -> the scoped list reports total=2.
	rec = doJSON(r, http.MethodPost, "/api/v1/schedules",
		`{"name":"第二条","cronExpr":"0 4 * * *","functionId":"fn.reset2","gameId":"demo","env":"prod"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// DB persistence checks: the payload stays verbatim and the actor comes from ctx.
	var row model.TaskSchedule
	require.NoError(t, db.First(&row, id).Error)
	assert.Equal(t, `{"k":1}`, row.Payload.String())
	assert.Equal(t, "tester", row.Actor)
	assert.Equal(t, 3, row.MaxFailedRuns)
	assert.Equal(t, model.ScheduleStatusActive, row.Status)

	// After filtering only the two demo rows remain; with id DESC the first is the newest.
	rec = doReq(r, http.MethodGet, "/api/v1/schedules?gameId=demo&env=prod&page=1&pageSize=10")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Equal(t, int64(2), list.Total)
	require.Len(t, list.Items, 2)
	assert.Equal(t, "第二条", list.Items[0].Name)
	assert.Equal(t, "*/5 * * * *", list.Items[1].CronExpr)
	assert.NotEmpty(t, list.Items[1].NextTriggerAt)
	assert.Equal(t, "tester", list.Items[1].Actor)

	// Without a filter: the ctx scope (demo/prod) applies.
	rec = doReq(r, http.MethodGet, "/api/v1/schedules")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(2), list.Total)

	// Paging: pageSize=1 returns a single item but total stays unchanged.
	rec = doReq(r, http.MethodGet, "/api/v1/schedules?page=1&pageSize=1")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(2), list.Total)
	assert.Len(t, list.Items, 1)

	// The status filter matches nothing.
	rec = doReq(r, http.MethodGet, "/api/v1/schedules?status=paused")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(0), list.Total)
}

func TestCreate_InvalidCron(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules",
		`{"name":"bad","cronExpr":"not-a-cron","functionId":"fn","gameId":"demo","env":"prod"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "cron")
}

func TestCreate_MissingRequiredField(t *testing.T) {
	r, _ := newTestEnv(t, true)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules",
		`{"name":"no-fn","cronExpr":"*/5 * * * *","gameId":"demo","env":"prod"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "validation_failed")
}

func TestCreate_MissingScope(t *testing.T) {
	r, _ := newTestEnv(t, false)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules",
		`{"name":"no-scope","cronExpr":"*/5 * * * *","functionId":"fn"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "gameId/env")
}

func TestSetStatus_ActivePausedRoundTrip(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)

	rec := doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/schedules/%d/status", id), `{"status":"paused"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Item ScheduleItem `json:"item"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, model.ScheduleStatusPaused, resp.Item.Status)
	assert.Empty(t, resp.Item.NextTriggerAt)

	rec = doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/schedules/%d/status", id), `{"status":"active"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, model.ScheduleStatusActive, resp.Item.Status)
	assert.NotEmpty(t, resp.Item.NextTriggerAt)

	rec = doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/schedules/%d/status", id), `{"status":"bogus"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(r, http.MethodPut, "/api/v1/schedules/99999/status", `{"status":"paused"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doJSON(r, http.MethodPut, "/api/v1/schedules/abc/status", `{"status":"paused"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTriggerNow(t *testing.T) {
	r, _ := newTestEnv(t, true)
	id := mustCreate(t, r)

	// Dispatcher with an empty registry: no agent is available, so a failed dispatch is
	// a service-side 503 (AppError SERVICE_UNAVAILABLE), not a 500 internal error.
	rec := doReq(r, http.MethodPost, fmt.Sprintf("/api/v1/schedules/%d/trigger", id))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "service_unavailable")
	assert.Contains(t, rec.Body.String(), "no live agent")

	// A nonexistent schedule.
	rec = doReq(r, http.MethodPost, "/api/v1/schedules/99999/trigger")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// An invalid ID.
	rec = doReq(r, http.MethodPost, "/api/v1/schedules/abc/trigger")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRunLogs(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)

	slot := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	logs := []model.TaskScheduleRunLog{
		{ScheduleID: id, Slot: slot, TaskRunID: "run-1", Status: "dispatched", Message: "ok"},
		{ScheduleID: id, Slot: slot.Add(5 * time.Minute), TaskRunID: "run-2", Status: "failed", Message: "agent down"},
		{ScheduleID: id + 1, Slot: slot, TaskRunID: "run-3", Status: "dispatched"},
	}
	for i := range logs {
		require.NoError(t, db.Create(&logs[i]).Error)
	}

	rec := doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/schedules/%d/runs?page=1&pageSize=1", id))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp RunLogsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(2), resp.Total)
	require.Len(t, resp.Items, 1)
	// id DESC: the newest run, run-2, comes first.
	assert.Equal(t, "run-2", resp.Items[0].TaskRunID)
	assert.Equal(t, "failed", resp.Items[0].Status)
	assert.Equal(t, slot.Add(5*time.Minute).Format(time.RFC3339), resp.Items[0].Slot)

	rec = doReq(r, http.MethodGet, "/api/v1/schedules/abc/runs")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDelete(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)

	rec := doReq(r, http.MethodDelete, fmt.Sprintf("/api/v1/schedules/%d", id))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())

	var total int64
	require.NoError(t, db.Model(&model.TaskSchedule{}).Count(&total).Error)
	assert.Equal(t, int64(0), total)

	rec = doReq(r, http.MethodGet, "/api/v1/schedules")
	var list ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Equal(t, int64(0), list.Total)

	rec = doReq(r, http.MethodDelete, "/api/v1/schedules/abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestScheduleMutationsRequireScope(t *testing.T) {
	r, db := newTestEnv(t, true)
	id := mustCreate(t, r)
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "other", Env: "prod"})
	s := NewService(&svc.ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		Dispatcher:        dispatch.NewDispatcher(reg.NewStore()),
	})

	_, err := s.SetStatus(ctx, id, model.ScheduleStatusPaused)
	require.ErrorContains(t, err, "not found in game")
	_, err = s.RunLogs(ctx, id, 1, 10)
	require.ErrorContains(t, err, "not found in game")
	require.ErrorContains(t, s.Delete(ctx, id), "not found in game")
	_, err = s.TriggerNow(ctx, id)
	require.ErrorContains(t, err, "not found in game")

	items, _, err := model.NewTaskScheduleModel(db).List(context.Background(), model.ListSchedulesOptions{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, model.ScheduleStatusActive, items[0].Status)
}

func TestModelNotInitialized(t *testing.T) {
	h := NewHandler(NewService(&svc.ServiceContext{}))
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/schedules", nil)
	h.List(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
