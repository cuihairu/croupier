package task

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBufferV9(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func dispatchTaskRunWriterOfV9(svcCtx *svc.ServiceContext) *dispatch.TaskRunWriterAdapter {
	return dispatch.NewTaskRunWriterAdapter(model.NewTaskRunModel(svcCtx.DB))
}

// newHandlerWithRuntimeV9 构造注入 fake runtime 的 Handler，覆盖 service 报错分支。
func newHandlerWithRuntimeV9(t *testing.T, rt TaskRuntime) *Handler {
	t.Helper()
	return NewHandler(&Service{svcCtx: setupSvcCtx(t), runtime: rt})
}

func TestHandlerListV9_ServiceError(t *testing.T) {
	t.Parallel()
	handler := newHandlerWithRuntimeV9(t, &fakeRuntimeV9{listRunsErr: assert.AnError})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks?page=1&size=10", nil)

	handler.List(c)
	assert.GreaterOrEqual(t, w.Code, 400, "expected error status code, got %d", w.Code)
}

func TestHandlerStartV9_DispatchSuccess(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svcCtx.Dispatcher.SetTaskRunWriter(dispatchTaskRunWriterOfV9(svcCtx))
	caller := &fakeAgentSessionCaller{}
	svcCtx.Dispatcher.SetSessionResolver(&fakeAgentResolver{caller: caller})

	createTestFunction(t, svcCtx.DB, "player.ban", "Ban Player")
	svcCtx.RegistryStore.UpsertAgent(&registry.AgentSession{
		AgentID:  "agent-v9",
		GameID:   "test-game",
		Env:      "test-env",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]registry.FunctionMeta{
			"player.ban": {Enabled: true},
		},
	})

	adminCtx := svc.WithGameScope(seedAdminWithRole(t, svcCtx.DB, "root", "admin"), svc.GameScope{GameID: "test-game", Env: "test-env"})
	handler := NewHandler(NewService(svcCtx))

	body, _ := json.Marshal(StartRequest{FunctionID: "player.ban", Params: map[string]interface{}{"player": "p1"}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(adminCtx, "POST", "/tasks/start", newBufferV9(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Start(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp StartResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.TaskID)
	assert.Equal(t, tasks.StatusDispatching, resp.Status)
}

func TestHandlerDetailV9_Success(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	seedTaskRun(t, svc.svcCtx.DB, "t-detail-v9", "player.ban", tasks.StatusRunning)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-detail-v9", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-detail-v9"}}

	handler.Detail(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp DetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "t-detail-v9", resp.ID)
	assert.Equal(t, tasks.StatusRunning, resp.Status)
}

func TestHandlerEventsV9_BindQueryError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-1/events?afterSeq=not-a-number", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-1"}}

	handler.Events(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerEventsV9_AfterSeqNegativeRejected(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/t-1/events?after_seq=-1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-1"}}

	handler.Events(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerCancelV9_Success(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	seedTaskRun(t, svc.svcCtx.DB, "t-cancel-v9", "player.ban", tasks.StatusRunning)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/tasks/t-cancel-v9/cancel", nil)
	c.Params = []gin.Param{{Key: "id", Value: "t-cancel-v9"}}

	handler.Cancel(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var run model.TaskRun
	require.NoError(t, svc.svcCtx.DB.Where("task_id = ?", "t-cancel-v9").First(&run).Error)
	assert.Equal(t, tasks.StatusCancelRequested, run.Status)
}

func TestHandlerCancelByBodyV9_Success(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	seedTaskRun(t, svc.svcCtx.DB, "t-cancelbody-v9", "player.ban", tasks.StatusRunning)

	body, _ := json.Marshal(CancelBodyRequest{ID: "t-cancelbody-v9"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/tasks/cancel", newBufferV9(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CancelByBody(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var run model.TaskRun
	require.NoError(t, svc.svcCtx.DB.Where("task_id = ?", "t-cancelbody-v9").First(&run).Error)
	assert.Equal(t, tasks.StatusCancelRequested, run.Status)
}
