// Rounds out schedule edge cases: an explicit body scope, the currentActor
// fallback, metadata merging, buildItem LastTriggeredAt, and the model failure
// branches for run logs / delete / create.
package schedule

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var extraDBSeq atomic.Uint64

func newBareScheduleEnv(t *testing.T) (*gin.Engine, *gorm.DB, *svc.ServiceContext) {
	t.Helper()
	name := fmt.Sprintf("schedule_extra_%d", extraDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	svcCtx := &svc.ServiceContext{
		TaskScheduleModel: model.NewTaskScheduleModel(db),
		Dispatcher:        dispatch.NewDispatcher(reg.NewStore()),
	}
	h := NewHandler(NewService(svcCtx))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/schedules")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id/runs", h.RunLogs)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/trigger", h.TriggerNow)
	return r, db, svcCtx
}

const scopedCreateBody = `{"name":"n","cronExpr":"*/5 * * * *","functionId":"fn.reset","gameId":"demo","env":"prod"}`

// resolveScheduleScope: explicit body scope (no X-Game-ID context).
func TestCreateExplicitBodyScope(t *testing.T) {
	r, _, _ := newBareScheduleEnv(t)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// currentActor: falls back to an empty string with no username in context; also
// exercises the metadata merge branch.
func TestTriggerNowMetadataMergeAndActorFallback(t *testing.T) {
	r, db, svcCtx := newBareScheduleEnv(t)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	require.Equal(t, http.StatusOK, rec.Code)

	// Write metadata + last_triggered_at (covers the buildItem branch and the metadata merge)
	now := time.Now().UTC()
	require.NoError(t, db.Model(&model.TaskSchedule{}).Where("1=1").Updates(map[string]interface{}{
		"metadata":          model.JSON(`{"route":"hash"}`),
		"last_triggered_at": &now,
	}).Error)

	// No username in context (bare router) -> currentActor falls back to "";
	// the dispatcher has no agent -> it errors, but the metadata/actor assembly
	// branches are already covered
	rec = doJSON(r, http.MethodPost, "/api/v1/schedules/1/trigger", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	// buildItem: List carries LastTriggeredAt
	rec = doJSON(r, http.MethodGet, "/api/v1/schedules", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list struct {
		Items []struct {
			LastTriggerAt string `json:"lastTriggerAt"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.NotEmpty(t, list.Items)
	assert.NotEmpty(t, list.Items[0].LastTriggerAt)
	_ = svcCtx
}

// RunLogs query fails (missing table) -> the handler error branch.
func TestRunLogsStoreError(t *testing.T) {
	r, db, _ := newBareScheduleEnv(t)
	require.NoError(t, db.Migrator().DropTable("task_schedule_run_logs"))
	rec := doJSON(r, http.MethodGet, "/api/v1/schedules/1/runs", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// Delete / Create model failure branches.
func TestDeleteAndCreateModelFailure(t *testing.T) {
	r, db, _ := newBareScheduleEnv(t)
	require.NoError(t, db.Migrator().DropTable("task_schedules"))

	rec := doJSON(r, http.MethodDelete, "/api/v1/schedules/1", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	rec = doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// resolveScheduleScope: falls back to the X-Game-ID context when the body has no gameId/env.
func TestCreateFallsBackToContextScope(t *testing.T) {
	name := fmt.Sprintf("schedule_extra_%d", extraDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	svcCtx := &svc.ServiceContext{TaskScheduleModel: model.NewTaskScheduleModel(db)}
	h := NewHandler(NewService(svcCtx))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: "demo", Env: "prod"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	g := r.Group("/api/v1/schedules")
	g.POST("", h.Create)

	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", `{"name":"n","cronExpr":"*/5 * * * *","functionId":"fn.reset"}`)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
