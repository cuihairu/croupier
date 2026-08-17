package task

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupHandler(t *testing.T) (*Handler, *Service) {
	t.Helper()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)
	handler := NewHandler(svc)
	return handler, svc
}

// --- NewHandler ---

func TestNewHandlerV2(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.service)
}

// --- List handler ---

func TestHandler_List_BindQueryError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks?page=abc", nil)

	handler.List(c)
	// response.Error returns various status codes; just verify it responded
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_List_Success(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks?page=1&size=10", nil)

	handler.List(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Start handler ---

func TestHandler_Start_BindJSONError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/tasks/start", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Start(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_Start_EmptyFunctionID(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(StartRequest{FunctionID: "  "})
	c.Request, _ = http.NewRequest("POST", "/tasks/start", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Start(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

// --- Detail handler ---

func TestHandler_Detail_BindURIError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/", nil)

	handler.Detail(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_Detail_NotFound(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/missing", nil)
	c.Params = []gin.Param{{Key: "id", Value: "missing"}}

	handler.Detail(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

// --- Events handler ---

func TestHandler_Events_BindURIError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/events", nil)

	handler.Events(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_Events_EmptyID(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/%20/events", nil)
	c.Params = []gin.Param{{Key: "id", Value: "  "}}

	handler.Events(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_Events_AcceptsServerHTTPAfterSeqQuery(t *testing.T) {
	t.Parallel()
	handler, svc := setupHandler(t)
	run := seedTaskRun(t, svc.svcCtx.DB, "task-after-seq", "report.generate", tasks.StatusSucceeded)
	require.NoError(t, svc.runtime.AppendEvent(context.Background(), run.TaskID, tasks.EventProgress, 50, "halfway", []byte(`{"count":1}`)))
	require.NoError(t, svc.runtime.AppendEvent(context.Background(), run.TaskID, tasks.EventCompleted, 100, "done", []byte(`{"ok":true}`)))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/task-after-seq/events?after_seq=1", nil)
	c.Params = []gin.Param{{Key: "id", Value: "task-after-seq"}}

	handler.Events(c)
	require.Equal(t, http.StatusOK, w.Code)
	var response EventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	assert.Equal(t, int64(2), response.Items[0].Seq)
}

func TestHandler_Events_RejectsInvalidServerHTTPAfterSeqQuery(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/task-1/events?after_seq=not-a-number", nil)
	c.Params = []gin.Param{{Key: "id", Value: "task-1"}}

	handler.Events(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Cancel handler ---

func TestHandler_Cancel_BindURIError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/tasks/cancel", nil)

	handler.Cancel(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

// --- CancelByBody handler ---

func TestHandler_CancelByBody_BindJSONError(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/tasks/cancel", bytes.NewBufferString("not json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CancelByBody(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

func TestHandler_CancelByBody_EmptyID(t *testing.T) {
	t.Parallel()
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(CancelBodyRequest{ID: "  "})
	c.Request, _ = http.NewRequest("POST", "/tasks/cancel", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CancelByBody(c)
	assert.True(t, w.Code >= 400, "expected error status code, got %d", w.Code)
}

// --- decodePayload edge cases ---

func TestDecodePayloadV2(t *testing.T) {
	t.Parallel()

	// Empty data
	assert.Nil(t, decodePayload(nil))
	assert.Nil(t, decodePayload([]byte{}))

	// Valid JSON
	data := []byte(`{"key":"value"}`)
	result := decodePayload(data)
	assert.NotNil(t, result)

	// Invalid JSON falls back to string
	bad := []byte(`{not json}`)
	result = decodePayload(bad)
	str, ok := result.(string)
	assert.True(t, ok)
	assert.Equal(t, string(bad), str)

	// JSON array
	data = []byte(`[1,2,3]`)
	result = decodePayload(data)
	assert.NotNil(t, result)
}

// --- buildItem edge cases ---

func TestBuildItemV2(t *testing.T) {
	t.Parallel()

	// Only StartedAt set
	startedAt := time.Now()
	run := &model.TaskRun{
		TaskID:     "t-start-only",
		FunctionID: "f1",
		Status:     "running",
		StartedAt:  &startedAt,
	}
	item := buildItem(run)
	assert.NotEmpty(t, item.StartedAt)
	assert.Empty(t, item.FinishedAt)
	assert.Equal(t, int64(0), item.DurationMs)

	// Only FinishedAt set
	finishedAt := time.Now()
	run2 := &model.TaskRun{
		TaskID:     "t-finish-only",
		FunctionID: "f1",
		Status:     "succeeded",
		FinishedAt: &finishedAt,
	}
	item2 := buildItem(run2)
	assert.Empty(t, item2.StartedAt)
	assert.NotEmpty(t, item2.FinishedAt)
	assert.Equal(t, int64(0), item2.DurationMs)

	// Error message field
	run3 := &model.TaskRun{
		TaskID:       "t-err",
		FunctionID:   "f1",
		Status:       "failed",
		ErrorMessage: "something broke",
		GameID:       "g1",
		Env:          "prod",
		AgentID:      "agent1",
		InputPayload: datatypes.JSON([]byte("{}")),
	}
	item3 := buildItem(run3)
	assert.Equal(t, "something broke", item3.Error)
	assert.Equal(t, "g1", item3.GameID)
	assert.Equal(t, "prod", item3.Env)
	assert.Equal(t, "agent1", item3.AgentID)
}

// --- buildDetail edge cases ---

func TestBuildDetailV2(t *testing.T) {
	t.Parallel()

	// Without start/finish times
	run := &model.TaskRun{
		TaskID:        "t-detail-no-times",
		FunctionID:    "player.ban",
		Status:        "queued",
		Message:       "waiting",
		ErrorMessage:  "test error",
		GameID:        "g1",
		Env:           "dev",
		AgentID:       "a1",
		Actor:         "admin",
		Addr:          "10.0.0.1:9090",
		TraceID:       "trace-1",
		ResultPayload: datatypes.JSON([]byte(`{"result":"ok"}`)),
		InputPayload:  datatypes.JSON([]byte("{}")),
	}
	detail := buildDetail(run)
	assert.Equal(t, "t-detail-no-times", detail.ID)
	assert.Equal(t, "queued", detail.Status)
	assert.Equal(t, "waiting", detail.Message)
	assert.Equal(t, "test error", detail.Error)
	assert.Equal(t, "g1", detail.GameID)
	assert.Equal(t, "dev", detail.Env)
	assert.Equal(t, "a1", detail.AgentID)
	assert.Equal(t, "admin", detail.Actor)
	assert.Equal(t, "10.0.0.1:9090", detail.Addr)
	assert.Equal(t, "trace-1", detail.TraceID)
	assert.Equal(t, int64(0), detail.DurationMs)
	assert.NotNil(t, detail.Result) // has result payload
}

// --- List with GameID/Env filters ---

func TestListV2_FiltersByGameIDAndEnv(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-g1", "player.ban", tasks.StatusSucceeded)
	seedTaskRun(t, svcCtx.DB, "t-g2", "player.ban", tasks.StatusRunning)
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &ListRequest{GameID: "nonexistent", Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, len(resp.Items))

	resp, err = svc.List(context.Background(), &ListRequest{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp.Items))
}

// --- Detail empty ID ---

func TestDetailV2_EmptyID(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Detail(context.Background(), &DetailRequest{ID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "任务ID不能为空")
}

// --- Events empty ID ---

func TestEventsV2_EmptyID(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Events(context.Background(), &EventsRequest{ID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "任务ID不能为空")
}

// --- Cancel empty ID ---

func TestCancelV2_EmptyID(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	err := svc.Cancel(context.Background(), &CancelRequest{ID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "任务ID不能为空")
}

// --- Events Done status checks ---

func TestEventsV2_DoneStatuses(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)

	// Test each terminal status
	statuses := []string{tasks.StatusSucceeded, tasks.StatusFailed, tasks.StatusCancelled, tasks.StatusTimedOut}
	for _, status := range statuses {
		taskID := "t-done-" + status
		seedTaskRun(t, svcCtx.DB, taskID, "player.ban", status)
		svc := NewService(svcCtx)

		resp, err := svc.Events(context.Background(), &EventsRequest{ID: taskID})
		require.NoError(t, err)
		assert.True(t, resp.Done, "status %s should report done", status)
	}
}

func TestEventsV2_NotDone(t *testing.T) {
	t.Parallel()
	svcCtx := setupSvcCtx(t)
	seedTaskRun(t, svcCtx.DB, "t-running", "player.ban", tasks.StatusRunning)
	svc := NewService(svcCtx)

	resp, err := svc.Events(context.Background(), &EventsRequest{ID: "t-running"})
	require.NoError(t, err)
	assert.False(t, resp.Done)
}
