package executionlog

// #33: the execution-trace operator-aggregation options endpoint
// (GET /api/v1/execution-logs/operator-options).

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
	// Out-of-scope rows are excluded from the aggregation (constructed
	// directly, leaving the rows already landed by seedExecLog untouched)
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
	// ListRequest.Page is an int, so a non-numeric query value makes
	// ShouldBindQuery fail → 400
	req := httptest.NewRequest(http.MethodGet, "/api/v1/execution-logs/operator-options?page=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerOperatorOptions_ServiceError(t *testing.T) {
	// audit:read not granted: the service-side permission error exits through
	// the handler's unified error outlet → 403
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
	// req=nil takes the defaults branch (Mine=false → audit permission
	// required, already granted)
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

// Covers the model aggregation-query error branch of OperatorOptions: the
// fault is added only to queries on the execution_logs table -- the permission
// check also runs SQL, so a fail-all would blow up in RequireAnyPermission
// before ever reaching the aggregation query (unlike the mine=true
// permission-free path of TestService_List_StoreErrorGroupE).
func TestServiceOperatorOptions_StoreError(t *testing.T) {
	s, svcCtx, _ := newExecLogTestService(t, "alice")
	seedExecLog(t, svcCtx, "alice", "mail.send")
	grantAuditRead(t, svcCtx)

	// model.OperatorOptions aggregates with Scan (internally Rows → Row
	// processor, bypassing gorm:query), so the callback is registered on Row;
	// the Find/First in the permission check do not go through that processor
	// and are naturally unaffected.
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
