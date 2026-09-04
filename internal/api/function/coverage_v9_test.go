package function

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- failing stubs (V9) ---

type failingAuditStoreV9 struct{}

func (failingAuditStoreV9) Create(*audit.AuditRecord) error { return errors.New("audit boom") }
func (failingAuditStoreV9) Get(string) (*audit.AuditRecord, error) {
	return nil, errors.New("audit boom")
}
func (failingAuditStoreV9) List(audit.AuditFilter, audit.AuditPage) ([]*audit.AuditRecord, int, error) {
	return nil, 0, errors.New("audit boom")
}
func (failingAuditStoreV9) Delete(string) error { return errors.New("audit boom") }
func (failingAuditStoreV9) DeleteBefore(time.Time) (int64, error) {
	return 0, errors.New("audit boom")
}
func (failingAuditStoreV9) GetLatestRecord() (*audit.AuditRecord, error) {
	return nil, errors.New("audit boom")
}
func (failingAuditStoreV9) GetBySequence(int64) (*audit.AuditRecord, error) {
	return nil, errors.New("audit boom")
}
func (failingAuditStoreV9) GetChainRange(int64, int64) ([]*audit.AuditRecord, error) {
	return nil, errors.New("audit boom")
}
func (failingAuditStoreV9) GetStats(time.Time, time.Time) (*audit.AuditStats, error) {
	return nil, errors.New("audit boom")
}
func (failingAuditStoreV9) CountByFilter(audit.AuditFilter) (int64, error) {
	return 0, errors.New("audit boom")
}
func (failingAuditStoreV9) Export(audit.AuditFilter, string) ([]byte, error) {
	return nil, errors.New("audit boom")
}

type failingApprovalsStoreV9 struct{}

func (failingApprovalsStoreV9) List(approvals.Filter, approvals.Page) ([]*approvals.Approval, int, error) {
	return nil, 0, errors.New("approvals boom")
}
func (failingApprovalsStoreV9) Get(string) (*approvals.Approval, error) {
	return nil, errors.New("approvals boom")
}
func (failingApprovalsStoreV9) Approve(string, string) (*approvals.Approval, error) {
	return nil, errors.New("approvals boom")
}
func (failingApprovalsStoreV9) Reject(string, string, string) (*approvals.Approval, error) {
	return nil, errors.New("approvals boom")
}
func (failingApprovalsStoreV9) Create(*approvals.Approval) (*approvals.Approval, error) {
	return nil, errors.New("approvals boom")
}
func (failingApprovalsStoreV9) Update(*approvals.Approval) (*approvals.Approval, error) {
	return nil, errors.New("approvals boom")
}

// --- v9 fixtures ---

func attachTelemetryV9(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	tel, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		ServiceName:    "function-v9-test",
		ServiceVersion: "test",
		Environment:    "test",
		EnableTracing:  false,
		EnableMetrics:  false,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	svcCtx.Telemetry = tel
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })
}

func makeRegistrationWarningV9(key string) reg.FunctionRegistrationWarning {
	return reg.FunctionRegistrationWarning{
		Key:        key,
		GameID:     "demo",
		Env:        "prod",
		AgentID:    "agent-v9",
		FunctionID: "warn.fn",
		Version:    "1.0.0",
		Code:       "schema_mismatch",
		Message:    "v9 warning message",
	}
}

func agentSessionV9(gameID, env, agentID string, fnIDs ...string) *reg.AgentSession {
	fns := make(map[string]reg.FunctionMeta, len(fnIDs))
	for _, id := range fnIDs {
		fns[id] = reg.FunctionMeta{Enabled: true}
	}
	return &reg.AgentSession{
		AgentID:   agentID,
		GameID:    gameID,
		Env:       env,
		Addr:      "127.0.0.1:9500",
		Functions: fns,
		ExpireAt:  time.Now().Add(5 * time.Minute),
		LastSeen:  time.Now(),
	}
}

func providerSessionV9(gameID, providerID string, fnIDs ...string) reg.ProviderSession {
	return reg.ProviderSession{
		ProviderID:  providerID,
		GameID:      gameID,
		FunctionIDs: fnIDs,
		Addr:        "127.0.0.1:9600",
	}
}

func doRequestV9(t *testing.T, r *gin.Engine, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// --- service.SvcCtx ---

func TestServiceSvcCtx_V9(t *testing.T) {
	svcCtx := &svc.ServiceContext{}
	assert.Same(t, svcCtx, NewService(svcCtx).SvcCtx())
}

// --- handler service-error branches ---

func TestHandlerServiceErrorBranches_V9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("FunctionsList DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions", "")
		h.FunctionsList(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("FunctionDetail not found", func(t *testing.T) {
		f := newInvokeFixture(t)
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/ghost/detail", "")
		ctx.Params = gin.Params{{Key: "id", Value: "ghost"}}
		h.FunctionDetail(ctx)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("FunctionAnalytics DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/f/analytics", "")
		ctx.Params = gin.Params{{Key: "id", Value: "f"}}
		h.FunctionAnalytics(ctx)
		assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("FunctionDelete DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/delete", `{"functionId":"f1"}`)
		h.FunctionDelete(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("FunctionHistory DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/f/history", "")
		ctx.Params = gin.Params{{Key: "id", Value: "f"}}
		h.FunctionHistory(ctx)
		assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("FunctionInvoke missing scope", func(t *testing.T) {
		f := newInvokeFixture(t)
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/f/invoke", `{"id":"f"}`)
		h.FunctionInvoke(ctx)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("FunctionPermissions DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("function_permissions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/f/permissions", "")
		h.FunctionPermissions(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("FunctionPermissionsUpdate DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("function_permissions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/f/permissions", `{"permissions":[]}`)
		h.FunctionPermissionsUpdate(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("Descriptors DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("function_descriptors"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/descriptors", "")
		h.Descriptors(ctx)
		assert.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
	})

	t.Run("BatchUpdateFunctions DB error", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.db.Migrator().DropTable("functions"))
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodPost, "/api/v1/functions/batch", `{"functionIds":["f1"],"enabled":true}`)
		h.BatchUpdateFunctions(ctx)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

// --- warning handlers ---

func TestWarningHandlers_V9(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("nil store returns not found", func(t *testing.T) {
		h := NewHandler(NewService(&svc.ServiceContext{}))
		r := gin.New()
		r.DELETE("/warnings/:key", h.WarningDelete)
		r.DELETE("/warnings", h.WarningDeleteAll)
		r.POST("/warnings/:key/read", h.WarningMarkRead)
		r.POST("/warnings/read-all", h.WarningMarkAllRead)

		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodDelete, "/warnings/k1").Code)
		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodDelete, "/warnings").Code)
		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodPost, "/warnings/k1/read").Code)
		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodPost, "/warnings/read-all").Code)
	})

	t.Run("missing uri key is a bad request", func(t *testing.T) {
		f := newInvokeFixture(t)
		h := NewHandler(NewService(f.svcCtx))
		ctx, rec := newFunctionTestContext(http.MethodDelete, "/api/v1/functions/warnings/", "")
		h.WarningDelete(ctx)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		ctx, rec = newFunctionTestContext(http.MethodPost, "/api/v1/functions/warnings//read", "")
		h.WarningMarkRead(ctx)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("delete unknown key returns not found", func(t *testing.T) {
		f := newInvokeFixture(t)
		h := NewHandler(NewService(f.svcCtx))
		r := gin.New()
		r.DELETE("/warnings/:key", h.WarningDelete)
		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodDelete, "/warnings/ghost").Code)
	})

	t.Run("delete and mark existing key succeed", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.store.UpsertRegistrationWarning(context.Background(), makeRegistrationWarningV9("k-v9")))
		h := NewHandler(NewService(f.svcCtx))
		r := gin.New()
		r.DELETE("/warnings/:key", h.WarningDelete)
		r.POST("/warnings/:key/read", h.WarningMarkRead)

		rec := doRequestV9(t, r, http.MethodPost, "/warnings/k-v9/read")
		assert.Equal(t, http.StatusOK, rec.Code)

		rec = doRequestV9(t, r, http.MethodDelete, "/warnings/k-v9")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "k-v9")

		// 已删除后再标记已读 → 404
		assert.Equal(t, http.StatusNotFound, doRequestV9(t, r, http.MethodPost, "/warnings/k-v9/read").Code)
	})

	t.Run("delete all and mark all read", func(t *testing.T) {
		f := newInvokeFixture(t)
		require.NoError(t, f.store.UpsertRegistrationWarning(context.Background(), makeRegistrationWarningV9("k-a")))
		require.NoError(t, f.store.UpsertRegistrationWarning(context.Background(), makeRegistrationWarningV9("k-b")))
		h := NewHandler(NewService(f.svcCtx))
		r := gin.New()
		r.DELETE("/warnings", h.WarningDeleteAll)
		r.POST("/warnings/read-all", h.WarningMarkAllRead)

		rec := doRequestV9(t, r, http.MethodPost, "/warnings/read-all")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"marked":2`)

		rec = doRequestV9(t, r, http.MethodDelete, "/warnings")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"deleted":2`)
	})
}

// --- functionsList metadata fallback ---

func TestFunctionsList_MetadataFallback_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()
	require.NoError(t, f.db.WithContext(ctx).Create(&model.Function{
		FunctionID: "meta.fn",
		Name:       "Meta Fn",
		GameID:     "demo",
		Status:     1,
		Metadata: map[string]interface{}{
			"resource":     "meta-resource",
			"version":      "9.9.9",
			"spec_format":  "openapi3.meta",
			"openapi_spec": map[string]interface{}{"openapi": "3.0.0"},
			"tags":         []interface{}{"t1", "t2"},
			"summary":      map[string]interface{}{"en": "meta summary"},
		},
	}).Error)

	resp, err := functionsList(ctx, f.svcCtx, &FunctionsListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	item := resp.Items[0]
	assert.Equal(t, "meta-resource", item.Resource)
	assert.Equal(t, "9.9.9", item.Version)
	assert.Equal(t, "openapi3.meta", item.SpecFormat)
	assert.NotNil(t, item.OpenAPISpec)
	assert.Equal(t, []string{"t1", "t2"}, item.Tags)
	assert.Equal(t, map[string]string{"en": "meta summary"}, item.Summary)
	assert.NotEmpty(t, item.CreatedAt)
	assert.NotEmpty(t, item.UpdatedAt)
}

// --- setFunctionEnabled update failure ---

func TestSetFunctionEnabled_UpdateError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()
	require.NoError(t, f.db.WithContext(ctx).Create(&model.Function{
		FunctionID: "blocked.fn", Name: "Blocked", Status: 1,
	}).Error)
	require.NoError(t, f.db.Exec(
		`CREATE TRIGGER v9_block_update BEFORE UPDATE ON functions BEGIN SELECT RAISE(ABORT, 'update blocked'); END`,
	).Error)

	err := functionDisable(ctx, f.svcCtx, &FunctionDisableRequest{FunctionId: "blocked.fn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update function")
}

// --- functionInvoke: telemetry / admin / permission errors ---

func TestFunctionInvoke_TelemetrySpan_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "teluser", "admin")
	attachTelemetryV9(t, f.svcCtx)

	// 成功路径：span + bridge 正常收尾
	f.registerAgent(t, "agent-tel", "tel.ok")
	caller := &fakeSessionCaller{invokePayload: []byte(`{"ok":1}`)}
	f.resolver.callers["agent-tel"] = caller
	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("teluser"), &FunctionInvokeRequest{ID: "tel.ok"})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// 失败路径：deferred span 记录错误事件
	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("teluser"), &FunctionInvokeRequest{ID: "ghost.fn"})
	require.Error(t, err)
}

func TestFunctionInvoke_LoadAdminError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	// scope 存在但缺少 username → LoadCurrentAdmin 失败
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "demo", Env: "prod"})
	_, err := NewService(f.svcCtx).FunctionInvoke(ctx, &FunctionInvokeRequest{ID: "demo.fn"})
	require.Error(t, err)
}

func TestFunctionInvoke_PermissionIDsError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "permuser", "operator")
	require.NoError(t, f.db.Migrator().DropTable("role_permissions"))

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("permuser"), &FunctionInvokeRequest{ID: "demo.fn"})
	require.Error(t, err)
}

func TestFunctionInvoke_MetadataCleanupAndHashRoute_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "metauser", "admin")
	f.registerAgent(t, "agent-meta", "meta.fn")
	caller := &fakeSessionCaller{invokePayload: []byte(`{}`)}
	f.resolver.callers["agent-meta"] = caller

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("metauser"), &FunctionInvokeRequest{
		ID:      "meta.fn",
		Route:   "hash",
		HashKey: "player-42",
		Metadata: map[string]string{
			"":       "dropped",
			"blank":  "  ",
			" ":      "dropped",
			"custom": "kept",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "kept", resp.ExecutionMetadata["custom"])
	assert.NotContains(t, resp.ExecutionMetadata, "blank")
	assert.Equal(t, "player-42", resp.ExecutionMetadata["hashKey"])

	require.Len(t, caller.requests, 1)
	assert.Equal(t, "player-42", caller.requests[0].GetMetadata()["hashKey"])
	assert.Equal(t, "hash", caller.requests[0].GetMetadata()["route"])
}

func TestFunctionInvoke_AsyncAndBroadcastErrors_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "erruser", "admin")

	// async 无 agent → StartTaskRequest 失败
	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("erruser"), &FunctionInvokeRequest{
		ID: "ghost.fn", Mode: "async",
	})
	require.Error(t, err)
	require.NotNil(t, resp)

	// broadcast 无 agent → InvokeBroadcast 失败
	resp, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("erruser"), &FunctionInvokeRequest{
		ID: "ghost.fn", Route: "broadcast",
	})
	require.Error(t, err)
	require.NotNil(t, resp)
}

// --- validateInvokeRoute / timeout helpers ---

func TestValidateInvokeRoute_NilAndUnknown_V9(t *testing.T) {
	require.Error(t, validateInvokeRoute(nil))
	require.Error(t, validateInvokeRoute(&FunctionInvokeRequest{Route: "bogus"}))
	require.Error(t, validateInvokeRoute(&FunctionInvokeRequest{Route: "broadcast", Mode: "task"}))
}

func TestEffectiveTimeoutMs_Boundaries_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("nobody")

	// 负数显式值按 0 处理（不注入）
	assert.Equal(t, 0, effectiveTimeoutMs(ctx, f.svcCtx, &FunctionInvokeRequest{TimeoutMs: -5}))

	// nil svcCtx / nil DB → 0
	assert.Equal(t, 0, declaredTimeoutMs(ctx, nil, &FunctionInvokeRequest{ID: "f"}))
	assert.Equal(t, 0, declaredTimeoutMs(ctx, &svc.ServiceContext{}, &FunctionInvokeRequest{ID: "f"}))

	// game/env 缺失 → 0
	assert.Equal(t, 0, declaredTimeoutMs(ctx, f.svcCtx, &FunctionInvokeRequest{ID: "f"}))
	assert.Equal(t, 0, declaredTimeoutMs(ctx, f.svcCtx, &FunctionInvokeRequest{ID: "f", GameID: "demo"}))
}

// --- audit failures ---

func TestFunctionInvoke_AuditLogError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "audituser", "admin")
	f.svcCtx.AuditService = audit.NewAuditService(failingAuditStoreV9{}, nil)
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "audited.fn", &policymgr.Policy{
		FunctionID:   "audited.fn",
		RequireAudit: true,
	}))

	// 调用失败（无 agent）也会走审计记录路径，Log 失败仅打印
	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("audituser"), &FunctionInvokeRequest{ID: "audited.fn"})
	require.Error(t, err)
}

func TestFunctionInvoke_ApprovalAuditAndNotify_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "appruser", "admin")
	f.svcCtx.NotifyService = notify.New(nil, nil)
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "notify.fn", &policymgr.Policy{
		FunctionID:      "notify.fn",
		RequireApproval: true,
		RequireAudit:    true,
	}))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("appruser"), &FunctionInvokeRequest{
		ID: "notify.fn", Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	assert.True(t, resp.ApprovalRequired)
	assert.NotEmpty(t, resp.ApprovalID)

	stored, err := f.approvals.Get(resp.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, "lb", stored.Route)
}

func TestFunctionInvoke_ApprovalAuditLogError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "apprfail", "admin")
	f.svcCtx.AuditService = audit.NewAuditService(failingAuditStoreV9{}, nil)
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(context.Background(), "appr.fail.fn", &policymgr.Policy{
		FunctionID:      "appr.fail.fn",
		RequireApproval: true,
		RequireAudit:    true,
	}))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("apprfail"), &FunctionInvokeRequest{ID: "appr.fail.fn"})
	require.NoError(t, err)
	assert.True(t, resp.ApprovalRequired)
}

func TestCreateFunctionApproval_Direct_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()

	// 存储失败 → 包装错误
	f.svcCtx.ApprovalsStore = failingApprovalsStoreV9{}
	_, err := createFunctionApproval(ctx, f.svcCtx, &FunctionInvokeRequest{ID: "store.fail"}, []byte(`{}`), nil, &policymgr.Policy{RequireApproval: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create approval")

	// 正常路径：无 AdminModel（接收人查询失败静默跳过）+ NotifyService 空配置
	f2 := newInvokeFixture(t)
	f2.svcCtx.ApprovalsStore = approvals.NewMemStore()
	f2.svcCtx.AdminModel = nil
	f2.svcCtx.NotifyService = notify.New(nil, nil)
	admin := &model.Admin{Username: "direct-user"}
	id, err := createFunctionApproval(ctx, f2.svcCtx, &FunctionInvokeRequest{
		ID:       "direct.fn",
		GameID:   "demo",
		Env:      "prod",
		Metadata: map[string]string{"k": "v"},
	}, []byte(`{"p":1}`), admin, &policymgr.Policy{RequireApproval: true})
	require.NoError(t, err)
	require.NotEmpty(t, id)
}

// --- instance listing edge cases ---

func TestFunctionInstances_NilSessionAndScopeMismatch_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("nobody") // demo/prod

	// nil 会话跳过 + scope 不匹配跳过
	f.store.AgentsUnsafe()["nil-sess"] = nil
	require.NoError(t, f.store.UpsertAgent(agentSessionV9("other-game", "prod", "wrong-game-agent", "demo.echo")))
	require.NoError(t, f.store.UpsertAgent(agentSessionV9("demo", "staging", "wrong-env-agent", "demo.echo")))
	require.NoError(t, f.store.UpsertAgent(agentSessionV9("demo", "prod", "good-agent", "demo.echo")))

	resp, err := NewService(f.svcCtx).FunctionInstances(ctx, &FunctionInstancesRequest{ID: "demo.echo"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "good-agent", resp.Items[0].AgentId)
	assert.Equal(t, "active", resp.Items[0].Status)
}

func TestFunctionInstancesAll_NilSessionAndProviderMismatch_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("nobody") // demo/prod

	f.store.AgentsUnsafe()["nil-sess"] = nil
	require.NoError(t, f.store.UpsertAgent(agentSessionV9("other", "prod", "wrong-agent", "x.fn")))

	// 会话 scope 匹配但 provider 的 game 不匹配 → provider 条目跳过，回落 agent 级
	mixed := agentSessionV9("demo", "prod", "mixed-agent", "provider-only.fn")
	mixed.Providers = []reg.ProviderSession{providerSessionV9("other", "svc-x", "provider-only.fn")}
	require.NoError(t, f.store.UpsertAgent(mixed))

	// 正常 provider 条目
	good := agentSessionV9("demo", "prod", "good-agent", "claimed.fn")
	good.Providers = []reg.ProviderSession{providerSessionV9("demo", "svc-good", "claimed.fn")}
	require.NoError(t, f.store.UpsertAgent(good))

	resp, err := NewService(f.svcCtx).FunctionInstancesAll(ctx, &FunctionInstancesAllRequest{})
	require.NoError(t, err)

	byFn := map[string]FunctionInstanceSummary{}
	for _, item := range resp.Instances {
		byFn[item.FunctionID] = item
	}
	require.Len(t, resp.Instances, 2)
	assert.Equal(t, "svc-good", byFn["claimed.fn"].ServiceID)
	assert.Empty(t, byFn["provider-only.fn"].ServiceID, "不匹配 scope 的 provider 条目应回落为 agent 级")
}

// --- permissions / warnings / batch error branches ---

func TestFunctionPermissions_ListError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("function_permissions"))
	_, err := NewService(f.svcCtx).FunctionPermissions(context.Background(), &FunctionPermissionsRequest{ID: "f"})
	require.Error(t, err)
}

func TestFunctionWarnings_WithItems_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := f.ctxFor("nobody")
	require.NoError(t, f.store.UpsertRegistrationWarning(ctx, makeRegistrationWarningV9("warn-1")))
	require.NoError(t, f.store.UpsertRegistrationWarning(ctx, reg.FunctionRegistrationWarning{
		GameID:     "other-game",
		Env:        "prod",
		AgentID:    "a2",
		FunctionID: "other.fn",
		Code:       "boom",
		Message:    "m2",
	}))

	resp, err := NewService(f.svcCtx).FunctionWarnings(ctx, &FunctionWarningsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	item := resp.Items[0]
	assert.Equal(t, "warn-1", item.Key)
	assert.Equal(t, "demo", item.GameID)
	assert.Equal(t, "schema_mismatch", item.Code)
	assert.Equal(t, 1, item.Count)
	assert.NotEmpty(t, item.FirstSeen)
	assert.NotEmpty(t, item.LastSeen)
}

func TestBatchDeleteFunctions_FailureBranch_V9(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("functions"))

	resp, err := NewService(f.svcCtx).BatchDeleteFunctions(context.Background(), &BatchDeleteFunctionsRequest{
		FunctionIds: []string{"a", "b"},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Deleted)
	assert.Equal(t, []string{"a", "b"}, resp.Failed)
}

func TestEnforceInvokePermission_ErrorBranches_V9(t *testing.T) {
	// FunctionModel 未初始化 → 明确 forbidden
	err := enforceInvokePermission(&svc.ServiceContext{}, []string{"viewer"}, nil, "f", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "函数权限模型未初始化")

	// ListPermissions 数据库错误 → 透传
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("function_permissions"))
	err = enforceInvokePermission(f.svcCtx, []string{"viewer"}, nil, "f", "", "")
	require.Error(t, err)
}

// --- enforceFunctionPolicy branches ---

func TestEnforceFunctionPolicy_RiskFromOpenAPI_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()

	// x-risk-level 字符串 → 高风险默认策略（需审批）
	op := openapi3.NewOperation()
	op.Extensions = map[string]interface{}{"x-risk-level": "high"}
	require.NoError(t, f.store.UpsertOpenAPI("risk.fn", op))
	p, err := enforceFunctionPolicy(ctx, f.svcCtx, "risk.fn", []string{"admin"})
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, p.RequireApproval)

	// 扩展值非字符串 → 断言失败回落 medium
	op2 := openapi3.NewOperation()
	op2.Extensions = map[string]interface{}{"x-risk-level": 42}
	require.NoError(t, f.store.UpsertOpenAPI("risk.num", op2))
	p, err = enforceFunctionPolicy(ctx, f.svcCtx, "risk.num", []string{"admin"})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestEnforceFunctionPolicy_GetPolicyError_V9(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("function_policies"))

	// 策略读取失败不阻断调用：返回 (nil, nil)
	p, err := enforceFunctionPolicy(context.Background(), f.svcCtx, "any.fn", []string{"admin"})
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestEnforceFunctionPolicy_CasbinBranch_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()

	// 非 admin 用户带 function:invoke 权限
	admin := &model.Admin{Username: "casbin9", Nickname: "casbin9", Status: 1}
	require.NoError(t, f.svcCtx.AdminModel.Create(ctx, admin, "pw"))
	role := &model.Role{Name: "casbin_role", Category: "test"}
	require.NoError(t, f.svcCtx.RoleModel.Create(ctx, role))
	require.NoError(t, f.svcCtx.AdminModel.AssignRole(ctx, admin.ID, role.ID))
	require.NoError(t, f.svcCtx.RoleModel.ReplacePermissions(ctx, role.ID, []string{"function:invoke"}))

	userCtx := context.WithValue(context.Background(), "username", "casbin9")

	// 角色匹配 → 通过 Casbin + AllowedRoles 校验，返回策略
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(ctx, "casbin.ok", &policymgr.Policy{
		FunctionID:   "casbin.ok",
		AllowedRoles: []string{"casbin_role"},
	}))
	p, err := enforceFunctionPolicy(userCtx, f.svcCtx, "casbin.ok", []string{"casbin_role"})
	require.NoError(t, err)
	require.NotNil(t, p)

	// 角色不匹配 → forbidden（函数级角色限制优先于 Casbin）
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(ctx, "casbin.deny", &policymgr.Policy{
		FunctionID:   "casbin.deny",
		AllowedRoles: []string{"admin"},
	}))
	_, err = enforceFunctionPolicy(userCtx, f.svcCtx, "casbin.deny", []string{"casbin_role"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权调用该函数")
}

func TestEnforceFunctionPolicy_FallbackMatch_V9(t *testing.T) {
	f := newInvokeFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svcCtx.PolicyManager.SetOverride(ctx, "fb.fn", &policymgr.Policy{
		FunctionID:   "fb.fn",
		AllowedRoles: []string{"viewer"},
	}))

	// 无 AdminModel → 简单角色匹配分支
	bare := &svc.ServiceContext{DB: f.db, PolicyManager: f.svcCtx.PolicyManager, RegistryStore: f.store}
	p, err := enforceFunctionPolicy(ctx, bare, "fb.fn", []string{"viewer"})
	require.NoError(t, err)
	require.NotNil(t, p)
}
