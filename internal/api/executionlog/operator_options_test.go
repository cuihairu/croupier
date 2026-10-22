package executionlog

// #33：执行留痕操作人聚合选项端点（GET /api/v1/execution-logs/operator-options）。

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

func newOperatorOptionsRouter(t *testing.T, username string) *gin.Engine {
	t.Helper()
	s, _, _ := newExecLogTestService(t, username)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(userMiddleware(username))
	r.GET("/api/v1/execution-logs/operator-options", NewHandler(s).OperatorOptions)
	return r
}

func TestHandlerOperatorOptions_BindError(t *testing.T) {
	r := newOperatorOptionsRouter(t, "alice")
	// ListRequest.Page 为 int，非数字查询值使 ShouldBindQuery 失败 → 400
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/operator-options?page=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerOperatorOptions_ServiceError(t *testing.T) {
	// 未授予 audit:read：service 翼权限错经 handler 统一错误出口 → 403
	r := newOperatorOptionsRouter(t, "alice")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/operator-options", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestServiceOperatorOptions_ModelUnavailable(t *testing.T) {
	s, _, ctx := newExecLogTestService(t, "alice")
	s.svcCtx.ExecutionLogModel = nil
	_, err := s.OperatorOptions(ctx, &ListRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution log model unavailable")
}

func TestServiceOperatorOptions_NilRequest(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	grantAuditRead(t, svcCtx)
	// req=nil 走默认值分支（Mine=false → 需要审计权限，已授予）
	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "alice"),
		svc.GameScope{GameID: "demo-game", Env: "development"})
	resp, err := s.OperatorOptions(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestServiceOperatorOptions_InvalidTimeParams(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	grantAuditRead(t, svcCtx)
	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "alice"),
		svc.GameScope{GameID: "demo-game", Env: "development"})
	_, err := s.OperatorOptions(ctx, &ListRequest{From: "not-a-time"})
	require.Error(t, err)
	_, err = s.OperatorOptions(ctx, &ListRequest{To: "2026-13-99"})
	require.Error(t, err)
}

// 覆盖 OperatorOptions 的模型聚合查询错误分支：只对 execution_logs 表的
// 查询加错——权限检查也走 SQL，fail-all 会先炸在 RequireAnyPermission
// 而到不了聚合查询（区别于 TestService_List_StoreErrorGroupE 的 mine=true
// 免权限路径）。
func TestServiceOperatorOptions_StoreError(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	grantAuditRead(t, svcCtx)

	// model.OperatorOptions 用 Scan 聚合（内部走 Rows → Row processor，不经
	// gorm:query），故注册在 Row 上；权限检查的 Find/First 不走此 processor，
	// 天然不受影响。
	require.NoError(t, svcCtx.DB.Callback().Row().Before("gorm:row").
		Register("opopts_fail_row", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("forced query failure"))
		}))
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Row().Remove("opopts_fail_row") })
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Query().Remove("opopts_fail_query") })

	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "alice"),
		svc.GameScope{GameID: "demo-game", Env: "development"})
	resp, err := s.OperatorOptions(ctx, &ListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}
