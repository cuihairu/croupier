package task

// #23：/tasks actor 过滤下推（此前 ListRequest 无 Actor 字段，前端发的参数
// 被服务端静默丢弃）+ 操作者聚合选项端点（GET /api/v1/tasks/operator-options）。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedTaskRunWithActor 与既有 seedTaskRun 的差别仅在 actor 维度（#23 聚合
// 与过滤的测试面）；不复用既有签名是为避免同包 helper 重名。
func seedTaskRunWithActor(t *testing.T, svcCtx *svc.ServiceContext, taskID, functionID, actor, status string) {
	t.Helper()
	require.NoError(t, model.NewTaskRunModel(svcCtx.DB).Create(context.Background(), &model.TaskRun{
		TaskID: taskID, FunctionID: functionID,
		Status: status, Actor: actor,
	}))
}

func TestTaskList_ActorFilterPushedDown(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRunWithActor(t, svcCtx, "t-1", "fn.a", "alice", "succeeded")
	seedTaskRunWithActor(t, svcCtx, "t-2", "fn.a", "bob", "succeeded")

	s := NewService(svcCtx)
	resp, err := s.List(context.Background(), &ListRequest{Actor: "alice"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "alice", resp.Items[0].Actor)
	assert.Equal(t, 1, resp.Total)
}

func TestOperatorOptions_AggregatesDistinctActors(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRunWithActor(t, svcCtx, "t-1", "fn.a", "alice", "succeeded")
	seedTaskRunWithActor(t, svcCtx, "t-2", "fn.a", "alice", "failed")
	seedTaskRunWithActor(t, svcCtx, "t-3", "fn.b", "bob", "succeeded")
	// 空 actor 排除
	seedTaskRunWithActor(t, svcCtx, "t-4", "fn.a", "", "succeeded")

	s := NewService(svcCtx)
	resp, err := s.OperatorOptions(context.Background(), &ListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "alice", resp.Items[0].Value)
	assert.Equal(t, int64(2), resp.Items[0].Count)
	assert.Equal(t, "bob", resp.Items[1].Value)
}

func TestHandler_OperatorOptions_OK(t *testing.T) {
	handler, svc := setupHandler(t)
	seedTaskRunWithActor(t, svc.svcCtx, "t-1", "fn.a", "alice", "succeeded")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/operator-options", nil)

	handler.OperatorOptions(c)
	require.Equal(t, http.StatusOK, w.Code)
	var resp OperatorOptionsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "alice", resp.Items[0].Value)
}

func TestOperatorOptions_NilRequest(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	s := NewService(svcCtx)
	resp, err := s.OperatorOptions(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// model.OperatorOptions 用 Scan 聚合，内部走 Rows() → Row processor（不经
// gorm:query），错误注入须注册在 Row 上（同 executionlog 包手法）。
func TestOperatorOptions_ModelQueryError(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	seedTaskRunWithActor(t, svcCtx, "t-1", "fn.a", "alice", "succeeded")

	require.NoError(t, svcCtx.DB.Callback().Row().Before("gorm:row").
		Register("task_opopts_fail_row", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("forced row query failure"))
		}))
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Row().Remove("task_opopts_fail_row") })

	s := NewService(svcCtx)
	resp, err := s.OperatorOptions(context.Background(), &ListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestHandler_OperatorOptions_BindError(t *testing.T) {
	handler, _ := setupHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// ListRequest.Page 为 int，非数字查询值使绑定失败 → 400
	c.Request, _ = http.NewRequest("GET", "/tasks/operator-options?page=abc", nil)
	handler.OperatorOptions(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_OperatorOptions_ServiceError(t *testing.T) {
	handler, svc := setupHandler(t)
	seedTaskRunWithActor(t, svc.svcCtx, "t-1", "fn.a", "alice", "succeeded")
	require.NoError(t, svc.svcCtx.DB.Callback().Row().Before("gorm:row").
		Register("task_opopts_fail_row_handler", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("forced row query failure"))
		}))
	t.Cleanup(func() { _ = svc.svcCtx.DB.Callback().Row().Remove("task_opopts_fail_row_handler") })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/tasks/operator-options", nil)
	handler.OperatorOptions(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
