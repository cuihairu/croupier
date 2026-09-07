package function

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

func finalFnDestIs[T any](tx *gorm.DB) bool {
	if tx.Statement == nil || tx.Statement.Dest == nil {
		return false
	}
	switch tx.Statement.Dest.(type) {
	case *T, *[]T, *[]*T:
		return true
	}
	return false
}

// failingApprovalsStore 让 Create 报错，其余委托 MemStore。
type failingApprovalsStore struct {
	*approvals.MemStore
	failCreate bool
}

func (s *failingApprovalsStore) Create(a *approvals.Approval) (*approvals.Approval, error) {
	if s.failCreate {
		return nil, errors.New("approval store unavailable")
	}
	return s.MemStore.Create(a)
}

func finalHandlerCtx(method, target, body string, reqCtx context.Context, uriID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if reqCtx != nil {
		req = req.WithContext(reqCtx)
	}
	c.Request = req
	if uriID != "" {
		c.Params = gin.Params{{Key: "id", Value: uriID}}
	}
	return c, rec
}

// ---------------------------------------------------------------------------
// handler 成功路径（response.Success 行）
// ---------------------------------------------------------------------------

func TestFinalHandler_FunctionDetail_Success(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.detail", "Detail")
	h := NewHandler(NewService(svcCtx))

	c, rec := finalHandlerCtx(http.MethodGet, "/api/v1/functions/detail", "", context.Background(), "demo.detail")
	h.FunctionDetail(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFinalHandler_FunctionAnalytics_Success(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.stats", "Stats")
	h := NewHandler(NewService(svcCtx))

	c, rec := finalHandlerCtx(http.MethodGet, "/api/v1/functions/analytics", "", context.Background(), "demo.stats")
	h.FunctionAnalytics(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFinalHandler_FunctionHistory_Success(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.hist", "Hist")
	h := NewHandler(NewService(svcCtx))

	c, rec := finalHandlerCtx(http.MethodGet, "/api/v1/functions/history", "", context.Background(), "demo.hist")
	h.FunctionHistory(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"items"`)
}

func TestFinalHandler_Descriptors_Success(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	h := NewHandler(NewService(svcCtx))
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})

	c, rec := finalHandlerCtx(http.MethodGet, "/api/v1/functions/descriptors", "", ctx, "")
	h.Descriptors(c)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// 审批流走通 handler 成功响应；Route 非空覆盖 createFunctionApproval 的
// route 透传分支。
func TestFinalHandler_FunctionInvoke_ApprovalSuccess(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", finalRequireApprovalPolicy()))

	h := NewHandler(NewService(f.svcCtx))
	c, rec := finalHandlerCtx(http.MethodPost, "/api/v1/functions/invoke",
		`{"id":"danger.blast","payload":{"n":1},"route":"hash","hashKey":"k"}`, f.ctxFor("opuser"), "")
	h.FunctionInvoke(c)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "approvalRequired")
}

// ---------------------------------------------------------------------------
// helpers.go 分支
// ---------------------------------------------------------------------------

// functionsList：外层 FunctionModel.List 失败（第二次 functions 查询）。
func TestFinalFunctionsList_ModelListError(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	var listQueries int
	name := "test.fail.fn.list"
	require.NoError(t, svcCtx.DB.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if finalFnDestIs[model.Function](tx) {
				listQueries++
				if listQueries >= 2 {
					tx.AddError(errors.New("injected list failure"))
				}
			}
		}))
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Query().Remove(name) })

	resp, err := NewService(svcCtx).FunctionsList(context.Background(), &FunctionsListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// functionsList：契约 resource 为空时回落 FunctionModel metadata。
func TestFinalFunctionsList_ResourceFallbackFromMetadata(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.fallback", "Fallback")
	require.NoError(t, svcCtx.DB.Create(&model.FunctionContract{
		GameID:      "test-game",
		Env:         "prod",
		FunctionID:  "demo.fallback",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "", // 空 resource，触发回落
	}).Error)

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "prod"})
	resp, err := NewService(svcCtx).FunctionsList(ctx, &FunctionsListRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Items)
	for _, item := range resp.Items {
		if item.Id == "demo.fallback" {
			assert.Equal(t, "test", item.Resource)
		}
	}
}

// functionInvoke：审批存储失败。
func TestFinalFunctionInvoke_ApprovalStoreFailure(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", finalRequireApprovalPolicy()))

	f.svcCtx.ApprovalsStore = &failingApprovalsStore{MemStore: f.approvals, failCreate: true}

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "danger.blast", Payload: []byte(`{}`),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create approval request")
}

// 执行留痕：成功调用写入 OK 状态与响应体。
func TestFinalFunctionInvoke_ExecutionLogSuccess(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	f.svcCtx.ExecutionLogWriter = executionlog.NewWriter(f.db, executionlog.Config{})

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Result)
}

// 执行留痕：失败调用写入 Fail 状态与错误体。
func TestFinalFunctionInvoke_ExecutionLogFailure(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.boom")
	bad := &fakeSessionCaller{err: assert.AnError}
	f.resolver.callers["agent-1"] = bad
	f.svcCtx.ExecutionLogWriter = executionlog.NewWriter(f.db, executionlog.Config{})

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.boom", Payload: []byte(`{}`), Route: "targeted", TargetServiceID: "agent-1",
	})
	require.Error(t, err)
}

// 执行留痕：成功但结果为空 → responseBody 保持 nil。
func TestFinalFunctionInvoke_ExecutionLogEmptyResult(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.empty")
	f.caller = &fakeSessionCaller{}
	f.resolver.callers["agent-1"] = f.caller
	f.svcCtx.ExecutionLogWriter = executionlog.NewWriter(f.db, executionlog.Config{})

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.empty", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Result)
}

// 审批通知：NotifyService 已配置且 Actor 非空时收件人去重追加。
func TestFinalFunctionInvoke_ApprovalNotifyWithActor(t *testing.T) {
	f := newInvokeFixture(t)
	f.svcCtx.NotifyService = notify.New(nil, model.NewMessageModel(f.db))
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", finalRequireApprovalPolicy()))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "danger.blast", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ApprovalID)
}

func finalRequireApprovalPolicy() *policymgr.Policy {
	return &policymgr.Policy{
		FunctionID:       "danger.blast",
		AllowedRoles:     []string{},
		RequireApproval:  true,
		DefaultRiskLevel: "high",
	}
}

// dedupeStrings：空串过滤与去重。
func TestFinalDedupeStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, dedupeStrings([]string{"a", "", "b", "a", ""}))
	assert.Empty(t, dedupeStrings(nil))
}

// route 已提供时审批记录透传 route（非 lb 默认）。
func TestFinalCreateFunctionApproval_RoutePassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", finalRequireApprovalPolicy()))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "danger.blast", Payload: []byte(`{}`), Route: "broadcast",
	})
	require.NoError(t, err)
	stored, err := f.approvals.Get(resp.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "broadcast", stored.Route)
	assert.WithinDuration(t, time.Now(), stored.CreatedAt, time.Minute)
}

// functionsList：DB 函数 resource 为空 → metadata 回落。
func TestFinalFunctionsList_ResourceEmptyFallback(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "demo.nores",
		Name:       "NoRes",
		GameID:     "test-game",
		Status:     1,
		Resource:   "", // 空，触发 metadata 回落
		Metadata:   map[string]interface{}{"resource": "meta-res", "openapi_spec": map[string]interface{}{"openapi": "3.0.3"}},
	}).Error)

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "prod"})
	resp, err := NewService(svcCtx).FunctionsList(ctx, &FunctionsListRequest{})
	require.NoError(t, err)
	found := false
	for _, item := range resp.Items {
		if item.Id == "demo.nores" {
			found = true
			assert.Equal(t, "meta-res", item.Resource)
		}
	}
	assert.True(t, found, "demo.nores must appear in list")
}

// 审批通知：收件人查询失败时静默跳过（recipients=nil 分支）。
func TestFinalFunctionInvoke_ApprovalNotifyRecipientsError(t *testing.T) {
	f := newInvokeFixture(t)
	f.svcCtx.NotifyService = notify.New(nil, model.NewMessageModel(f.db))
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "danger.blast")
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "danger.blast", finalRequireApprovalPolicy()))

	name := "test.fail.admins.list"
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if finalFnDestIs[model.Admin](tx) && tx.Statement.Dest != nil {
				if _, ok := tx.Statement.Dest.(*[]model.Admin); ok {
					tx.AddError(errors.New("injected admins list failure"))
				}
			}
		}))
	t.Cleanup(func() { _ = f.db.Callback().Query().Remove(name) })

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "danger.blast", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ApprovalID)
}

// functionsList：openAPISpec 回落。DB 行被 status 过滤（logic 层），
// registry 运行时条目的 OpenAPISpec 为空，外层 dbIndex 仍含该函数。
func TestFinalFunctionsList_OpenAPISpecFallback(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "demo.off",
		Name:       "Off",
		GameID:     "test-game",
		Status:     0, // 禁用：logic 层 status 过滤后不进入 items
		Metadata:   map[string]interface{}{"openapi_spec": map[string]interface{}{"openapi": "3.0.3"}},
	}).Error)
	require.NoError(t, svcCtx.RegistryStore.UpsertAgent(&reg.AgentSession{
		AgentID:   "agent-off",
		GameID:    "test-game",
		Env:       "prod",
		Functions: map[string]reg.FunctionMeta{"demo.off": {Enabled: true}},
		ExpireAt:  time.Now().Add(5 * time.Minute),
	}))

	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "test-game", Env: "prod"})
	resp, err := NewService(svcCtx).FunctionsList(ctx, &FunctionsListRequest{Status: 1})
	require.NoError(t, err)
	found := false
	for _, item := range resp.Items {
		if item.Id == "demo.off" {
			found = true
			assert.NotEmpty(t, item.OpenAPISpec)
		}
	}
	assert.True(t, found, "runtime entry for demo.off must be listed")
}
