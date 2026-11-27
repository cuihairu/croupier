package ops

// GAME-010 slice 4：业务操作凭据运维面（GET 查询 / POST 取消确认）路由级用例。
// 作用域经 scope 中间件注入（生产为 GameDBMiddleware）；取消 invoker 注入
// stub（D5 派发语义在 gameops/functioninvoke 包已测，这里验 HTTP 契约映射：
// 200 视图 / 404 / 409 两种状态性冲突 / 503 通道不可用）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const durableOpsOpID = "cg:env-a:demo:018f3b27-5c2e-7b10-8000-0000000000ab"

// stubOpsCancelInvoker 按脚本回放信封或错误（HTTP 层注入）。
type stubOpsCancelInvoker struct {
	payload json.RawMessage
	err     error
	calls   int
}

func (s *stubOpsCancelInvoker) InvokeCancel(_ context.Context, _ string, _ []byte, _, _, _ string) (json.RawMessage, error) {
	s.calls++
	return s.payload, s.err
}

func newDurableOpsTestEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	svcCtx := &svc.ServiceContext{DB: db}

	r := gin.New()
	// 生产 GameDBMiddleware 的 scope 注入等价物 + 审计用户名。
	r.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: "cg", Env: "env-a"})
		c.Request = c.Request.WithContext(ctx)
		c.Set("username", "bob")
		c.Next()
	})
	h := NewDurableOpsHandler(svcCtx)
	r.GET("/business-operations/:operationId", h.DurableCredentialGet)
	r.POST("/business-operations/:operationId/cancel", h.DurableCredentialCancel)
	return r, db
}

func seedDurableCredential(t *testing.T, db *gorm.DB, outcome, cancelHint string, withResult bool) {
	t.Helper()
	store := gameops.NewCredentialStore(db)
	_, _, err := store.Register(context.Background(), "cg", "env-a", durableOpsOpID, "demo.durable", "fp-ops", []byte(`{"x":1}`), "alice", "demo.query", cancelHint)
	require.NoError(t, err)
	updates := map[string]interface{}{"outcome": outcome}
	if withResult {
		updates["game_receipt"] = "receipt-original"
		updates["result_json"] = model.JSON(`{"granted":true}`)
	}
	require.NoError(t, db.Model(&model.BusinessOperationCredential{}).Where("operation_id = ?", durableOpsOpID).Updates(updates).Error)
}

func TestDurableCredentialGet_ViewAndNotFound(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomeUnknown, "demo.cancel", true)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/business-operations/"+durableOpsOpID, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var view gameops.CredentialReconcile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
	assert.Equal(t, durableOpsOpID, view.OperationID)
	assert.Equal(t, model.BizOpOutcomeUnknown, view.Outcome)
	assert.Equal(t, "receipt-original", view.GameReceipt)
	assert.JSONEq(t, `{"granted":true}`, string(view.Result))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/business-operations/cg:env-a:demo:018f3b27-5c2e-7b10-8000-0000000000ff", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

// 游戏确认 canceled → 200 + 凭据迁移（D5 唯一 canceled 路径）。
func TestDurableCredentialCancel_GameConfirmed(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	stub := &stubOpsCancelInvoker{payload: json.RawMessage(`{"operationId":"` + durableOpsOpID + `","requestFingerprint":"fp-ops","outcome":"canceled","gameReceipt":"cancel-ok"}`)}
	restore := swapCancelInvoker(t, stub)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var view gameops.CredentialReconcile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
	assert.Equal(t, model.BizOpOutcomeCanceled, view.Outcome)
	assert.Equal(t, "cancel-ok", view.GameReceipt)
	assert.Equal(t, 1, stub.calls)

	var row model.BusinessOperationCredential
	require.NoError(t, db.Where("operation_id = ?", durableOpsOpID).First(&row).Error)
	assert.Equal(t, model.BizOpOutcomeCanceled, row.Outcome)
	_ = restore
}

// cancel_failed → 200 + 原 result 保留（响应如实回 cancel_failed，不谎称回滚；
// 信封自带回执不得覆盖原业务结果——补偿引用原凭据）。
func TestDurableCredentialCancel_CancelFailedKeepsResult(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomeUnknown, "demo.cancel", true)
	stub := &stubOpsCancelInvoker{payload: json.RawMessage(`{"operationId":"` + durableOpsOpID + `","requestFingerprint":"fp-ops","outcome":"cancel_failed","gameReceipt":"cancel-attempt","result":{"rolledBack":true}}`)}
	swapCancelInvoker(t, stub)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var view gameops.CredentialReconcile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
	assert.Equal(t, model.BizOpOutcomeCancelFailed, view.Outcome)
	assert.Equal(t, "receipt-original", view.GameReceipt, "视图回原回执")
	assert.JSONEq(t, `{"granted":true}`, string(view.Result), "视图回原 result")

	var row model.BusinessOperationCredential
	require.NoError(t, db.Where("operation_id = ?", durableOpsOpID).First(&row).Error)
	assert.Equal(t, "receipt-original", row.GameReceipt, "取消失败保留原回执")
	assert.JSONEq(t, `{"granted":true}`, string(row.ResultJSON), "取消失败保留原 result")
}

// 终态凭据（succeeded）→ 409 cancel_not_applicable，不派发（契约 §5：撤销
// 走补偿/撤销新 operationId）。
func TestDurableCredentialCancel_TerminalRefused(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomeSucceeded, "demo.cancel", true)
	stub := &stubOpsCancelInvoker{}
	swapCancelInvoker(t, stub)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "cancel_not_applicable")
	assert.Equal(t, 0, stub.calls)
}

// 未声明取消函数 → 409 cancel_not_supported，不派发。
func TestDurableCredentialCancel_NoHintRefused(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomePending, "", false)
	stub := &stubOpsCancelInvoker{}
	swapCancelInvoker(t, stub)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "cancel_not_supported")
	assert.Equal(t, 0, stub.calls)
}

// invoker 派发失败 → 500 原样透传，凭据原状（可重试确认，不谎报 cancel_failed）。
func TestDurableCredentialCancel_DispatchFailureIntact(t *testing.T) {
	r, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	stub := &stubOpsCancelInvoker{err: context.DeadlineExceeded}
	swapCancelInvoker(t, stub)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal_error")

	var row model.BusinessOperationCredential
	require.NoError(t, db.Where("operation_id = ?", durableOpsOpID).First(&row).Error)
	assert.Equal(t, model.BizOpOutcomePending, row.Outcome)
}

// Dispatcher 未装配 → 503 通道不可用（≠ 取消成功）。
func TestDurableCredentialCancel_NilDispatcher503(t *testing.T) {
	_, db := newDurableOpsTestEnv(t)
	seedDurableCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	// 生产 invoker（svcCtx.Dispatcher 为 nil）→ ConfirmCancellation 前的
	// handler 守卫先挡。
	h := NewDurableOpsHandler(&svc.ServiceContext{DB: db})
	r2 := gin.New()
	r2.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: "cg", Env: "env-a"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r2.POST("/business-operations/:operationId/cancel", h.DurableCredentialCancel)

	w := httptest.NewRecorder()
	r2.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/business-operations/"+durableOpsOpID+"/cancel", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "service_unavailable")
}

func swapCancelInvoker(t *testing.T, stub gameops.CancelInvoker) func() {
	t.Helper()
	original := newCancelInvoker
	newCancelInvoker = func(_ *svc.ServiceContext) gameops.CancelInvoker { return stub }
	t.Cleanup(func() { newCancelInvoker = original })
	return func() { newCancelInvoker = original }
}
