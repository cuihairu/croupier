package executionlog

// #33：执行留痕操作人聚合选项端点（GET /api/v1/execution-logs/operator-options）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorOptions_AggregatesScopedDistinctActors(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	seedExecLog(t, svcCtx, "bob", "player.ban")
	// scope 外不进聚合（直接构造，不动 seedExecLog 已落库的行）
	outside := model.ExecutionLog{GameID: "other-game", Env: "development", Source: "invoke",
		FunctionID: "mail.send", Actor: "carol", Status: "ok"}
	require.NoError(t, svcCtx.ExecutionLogModel.Create(context.Background(), &outside))
	grantAuditRead(t, svcCtx)

	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "alice"),
		svc.GameScope{GameID: "demo-game", Env: "development"})
	resp, err := s.OperatorOptions(ctx, &ListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, "alice", resp.Items[0].Value)
	assert.Equal(t, int64(2), resp.Items[0].Count)
	assert.Equal(t, "bob", resp.Items[1].Value)
}

func TestOperatorOptions_RequiresAuditPermission(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")

	_, err := s.OperatorOptions(context.WithValue(context.Background(), "username", "alice"), &ListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看执行留痕")
}

func TestHandlerOperatorOptions_OK(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	grantAuditRead(t, svcCtx)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userMiddleware("alice"))
	r.GET("/api/v1/execution-logs/operator-options", func(c *gin.Context) {
		c.Request = c.Request.WithContext(svc.WithGameScope(c.Request.Context(),
			svc.GameScope{GameID: "demo-game", Env: "development"}))
		NewHandler(s).OperatorOptions(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/operator-options", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp OperatorOptionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "alice", resp.Items[0].Value)
	assert.Equal(t, int64(1), resp.Items[0].Count)
}
