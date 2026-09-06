// 补齐 schedule 边界：显式 body scope、currentActor 回退、metadata 合并、
// buildItem LastTriggeredAt、运行日志/删除/创建的模型故障分支。
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

// resolveScheduleScope：显式 body scope（无 X-Game-ID 上下文）。
func TestCreateExplicitBodyScope(t *testing.T) {
	r, _, _ := newBareScheduleEnv(t)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// currentActor：无 username 上下文回退空串；metadata 合并分支。
func TestTriggerNowMetadataMergeAndActorFallback(t *testing.T) {
	r, db, svcCtx := newBareScheduleEnv(t)
	rec := doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	require.Equal(t, http.StatusOK, rec.Code)

	// 写入 metadata + last_triggered_at（覆盖 buildItem 分支与 metadata 合并）
	now := time.Now().UTC()
	require.NoError(t, db.Model(&model.TaskSchedule{}).Where("1=1").Updates(map[string]interface{}{
		"metadata":          model.JSON(`{"route":"hash"}`),
		"last_triggered_at": &now,
	}).Error)

	// 无 username 上下文（bare router）→ currentActor 回退 ""；
	// dispatcher 无 agent → 报错，但 metadata/actor 组装分支已覆盖
	rec = doJSON(r, http.MethodPost, "/api/v1/schedules/1/trigger", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	// buildItem：List 带 LastTriggeredAt
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

// RunLogs 查询失败（缺表）→ handler 错误分支。
func TestRunLogsStoreError(t *testing.T) {
	r, db, _ := newBareScheduleEnv(t)
	require.NoError(t, db.Migrator().DropTable("task_schedule_run_logs"))
	rec := doJSON(r, http.MethodGet, "/api/v1/schedules/1/runs", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

// Delete / Create 模型故障分支。
func TestDeleteAndCreateModelFailure(t *testing.T) {
	r, db, _ := newBareScheduleEnv(t)
	require.NoError(t, db.Migrator().DropTable("task_schedules"))

	rec := doJSON(r, http.MethodDelete, "/api/v1/schedules/1", "")
	assert.NotEqual(t, http.StatusOK, rec.Code)

	rec = doJSON(r, http.MethodPost, "/api/v1/schedules", scopedCreateBody)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// resolveScheduleScope：body 无 gameId/env 时回退 X-Game-ID 上下文。
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
