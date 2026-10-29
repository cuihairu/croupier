package task

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/tasks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listTaskHandlerResponse(t *testing.T, handler *Handler, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(ctx, http.MethodGet, "/tasks?page=1&size=20", nil)
	handler.List(c)
	return w
}

func TestTaskCenterListReturnsPaginatedEnvelope(t *testing.T) {
	t.Parallel()

	handler, taskSvc := setupHandler(t)
	seedTaskRun(t, taskSvc.svcCtx.DB, "task-list", "player.ban", tasks.StatusRunning)

	w := listTaskHandlerResponse(t, handler, context.Background())
	require.Equal(t, http.StatusOK, w.Code)

	var body response.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, int64(1), body.Total)
	assert.Equal(t, 1, body.Page)
	assert.Equal(t, 20, body.Size)
}

func TestTaskCenterDetailRequiresMatchingScope(t *testing.T) {
	t.Parallel()

	svcCtx := setupSvcCtx(t)
	run := seedTaskRun(t, svcCtx.DB, "task-scope", "player.ban", tasks.StatusRunning)
	run.GameID = "game-a"
	run.Env = "prod"
	require.NoError(t, svcCtx.DB.Save(run).Error)

	handler := NewHandler(&Service{svcCtx: svcCtx, runtime: &fakeRuntimeV9{getRun: run}})
	forbiddenCtx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(forbiddenCtx, http.MethodGet, "/tasks/task-scope", nil)
	c.Params = gin.Params{{Key: "id", Value: "task-scope"}}
	handler.Detail(c)

	require.Equal(t, http.StatusForbidden, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "permission_denied", body["error"])
	assert.Contains(t, body["message"], "无权访问该任务")
	// 无权调用方不得从 details 反推目标任务作用域（scope 泄露）。
	if details, ok := body["details"].(map[string]any); ok {
		assert.NotContains(t, details, "taskId")
		assert.NotContains(t, details, "taskScope")
	}
}

func TestTaskCenterDetailMapsRecordNotFoundToHTTP404(t *testing.T) {
	t.Parallel()

	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, "/tasks/missing", nil)
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	handler.Detail(c)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}
