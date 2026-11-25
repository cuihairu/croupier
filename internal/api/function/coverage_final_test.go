package function

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
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
// Helpers
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

// failingApprovalsStore makes Create error and delegates everything else to
// MemStore.
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
// Handler success paths (the response.Success lines)
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

// The approval flow reaches a handler success response; a non-empty Route
// covers the createFunctionApproval route pass-through branch.
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
// helpers.go branches
// ---------------------------------------------------------------------------

// functionsList: the outer FunctionModel.List fails (the second functions
// query).
func TestFinalFunctionsList_ModelListError(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	var listQueries int
	name := "test.fail.fn.list"
	require.NoError(t, svcCtx.DB.Callback().Query().Before("gorm:query").
		Register(name, func(tx *gorm.DB) {
			if finalFnDestIs[model.Function](tx) {
				listQueries++
				if listQueries >= 2 {
					_ = tx.AddError(errors.New("injected list failure"))
				}
			}
		}))
	t.Cleanup(func() { _ = svcCtx.DB.Callback().Query().Remove(name) })

	resp, err := NewService(svcCtx).FunctionsList(context.Background(), &FunctionsListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

// functionsList: falls back to FunctionModel metadata when the contract's
// resource is empty.
func TestFinalFunctionsList_ResourceFallbackFromMetadata(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	createTestFunction(t, svcCtx.DB, "demo.fallback", "Fallback")
	require.NoError(t, svcCtx.DB.Create(&model.FunctionContract{
		GameID:      "test-game",
		Env:         "prod",
		FunctionID:  "demo.fallback",
		Version:     "1.0.0",
		Enabled:     true,
		ResourceKey: "", // empty resource triggers the fallback
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

// functionInvoke: approval store failure.
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

// Execution log: a successful invocation writes the OK status with the
// response body.
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

// Execution log: a failed invocation writes the Fail status with the error
// body.
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

// Execution log: success with an empty result -> responseBody stays nil.
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

// Approval notification: when NotifyService is configured and Actor is
// non-empty, deduplicated recipients are appended.
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

// dedupeStrings: empty-string filtering and deduplication.
func TestFinalDedupeStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, dedupeStrings([]string{"a", "", "b", "a", ""}))
	assert.Empty(t, dedupeStrings(nil))
}

// When a route is provided, the approval record passes it through (not the
// lb default).
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

// Route semantics wiring (400-level): targeted without targetServiceId no
// longer silently falls back to lb, and broadcast with async no longer
// silently goes async — both are intercepted before policy/dispatch.
func TestFinalFunctionInvoke_RouteValidationWired(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{}`), Route: "targeted",
	})
	require.Error(t, err)
	var badReq *errorx.CodeError
	if !errors.As(err, &badReq) {
		t.Fatalf("targeted without targetServiceId should be 400 CodeError, got %T: %v", err, err)
	}
	assert.Equal(t, http.StatusBadRequest, badReq.Code)

	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{}`), Route: "broadcast", Mode: "async",
	})
	require.Error(t, err)
	if !errors.As(err, &badReq) {
		t.Fatalf("broadcast+async should be 400 CodeError, got %T: %v", err, err)
	}
	assert.Equal(t, http.StatusBadRequest, badReq.Code)
}

// functionsList: DB function has an empty resource -> metadata fallback.
func TestFinalFunctionsList_ResourceEmptyFallback(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "demo.nores",
		Name:       "NoRes",
		GameID:     "test-game",
		Status:     1,
		Resource:   "", // empty, triggers the metadata fallback
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

// Approval notification: silently skipped when the recipient query fails
// (recipients=nil branch).
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
					_ = tx.AddError(errors.New("injected admins list failure"))
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

// functionsList: openAPISpec fallback. The DB row is filtered out by status
// (logic layer), the registry runtime entry has an empty OpenAPISpec, and the
// outer dbIndex still contains the function.
func TestFinalFunctionsList_OpenAPISpecFallback(t *testing.T) {
	svcCtx := setupTestServiceContext(t)
	require.NoError(t, svcCtx.DB.Create(&model.Function{
		FunctionID: "demo.off",
		Name:       "Off",
		GameID:     "test-game",
		Status:     0, // disabled: filtered by the logic layer's status filter, not in items
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
