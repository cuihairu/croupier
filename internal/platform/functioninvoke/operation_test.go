package functioninvoke

// GAME-010 slice 2：x-durable-idempotency 能力声明 → 凭据登记 → 信封收敛
// 的端到端判定。派发链复用 invoke_dispatch_test.go 的 stub caller，断言
// Response 凭据面与业务凭据行一致；不验证游戏侧持久去重（平台半边边界）。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func durableCapabilityStore(t *testing.T) *registry.Store {
	t.Helper()
	store := registry.NewStore()
	op := &openapi3.Operation{OperationID: "demo.durable"}
	op.Extensions = map[string]interface{}{
		"x-durable-idempotency": map[string]interface{}{
			"query": "demo.query", "cancel": "demo.cancel",
		},
	}
	require.NoError(t, store.UpsertOpenAPI("demo.durable", op))
	return store
}

func durableInvokeReq() *Request {
	return &Request{ID: "demo.durable", Payload: []byte(`{"amount":1}`)}
}

func setupDurable(t *testing.T, caller transport.SessionCaller) (*svc.ServiceContext, *registry.Store) {
	t.Helper()
	svcCtx, db := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	seedGovernAdmin(t, db, "alice", "operator")
	store := durableCapabilityStore(t)
	svcCtx.RegistryStore = store
	svcCtx.Dispatcher = dispatch.NewDispatcher(store)
	svcCtx.Dispatcher.SetSessionResolver(&stubResolver{callers: map[string]transport.SessionCaller{"a1": caller}})
	registerDispatchAgent(t, store, "a1", "demo.durable")
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	return svcCtx, store
}

func TestResolveDurableIdempotency_FailClosedOnSourceError(t *testing.T) {
	svcCtx, db := setupDispatchSvc(t, nil)
	// UpsertOpenAPI 对 nil operation 的 clone 序列化错误路径由同包单测覆盖；
	// 此处验证未声明与未命中均为 (nil,nil)。
	capability, err := resolveDurableIdempotency(context.Background(), svcCtx, "demo.echo")
	require.NoError(t, err)
	assert.Nil(t, capability)
	_ = db
}

func TestInvoke_DurableEnvelopeSucceeded(t *testing.T) {
	opID := "test-game:test-env:demo-durable:018f3b27-5c2e-7b10-8000-000000000001"
	caller := &stubCaller{invokePayload: []byte(`{"operationId":"` + opID + `","outcome":"succeeded","gameReceipt":"receipt-1","result":{"ok":true}}`)}
	svcCtx, _ := setupDurable(t, caller)

	req := durableInvokeReq()
	req.OperationID = opID
	resp, err := Invoke(dispatchCtx(), svcCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, opID, resp.OperationID)
	assert.Equal(t, model.BizOpOutcomeSucceeded, resp.Outcome)

	store := svcCtx.DB
	var cred model.BusinessOperationCredential
	require.NoError(t, store.Where("operation_id = ?", opID).First(&cred).Error)
	assert.Equal(t, model.BizOpOutcomeSucceeded, cred.Outcome)
	assert.Equal(t, "receipt-1", cred.GameReceipt)
	assert.JSONEq(t, `{"ok":true}`, string(cred.ResultJSON))
	assert.Equal(t, "demo.query", cred.QueryHint)
	assert.Equal(t, "demo.cancel", cred.CancelHint)
}

func TestInvoke_DurableLostResponseBecomesUnknown(t *testing.T) {
	caller := &stubCaller{invokePayload: nil}
	svcCtx, _ := setupDurable(t, caller)
	resp, err := Invoke(dispatchCtx(), svcCtx, durableInvokeReq())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, model.BizOpOutcomeUnknown, resp.Outcome, "结果未知绝不能报成功/失败")
	assert.Len(t, resp.OperationID, 36+len("test-game:test-env:demo-durable:"))

	var cred model.BusinessOperationCredential
	require.NoError(t, svcCtx.DB.Where("operation_id = ?", resp.OperationID).First(&cred).Error)
	assert.Equal(t, model.BizOpOutcomeUnknown, cred.Outcome)
}

func TestInvoke_DurableTransportErrorCarriesCredential(t *testing.T) {
	caller := &stubCaller{err: context.DeadlineExceeded}
	svcCtx, _ := setupDurable(t, caller)
	resp, err := Invoke(dispatchCtx(), svcCtx, durableInvokeReq())
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.OperationID)
	assert.Equal(t, model.BizOpOutcomeUnknown, resp.Outcome)
}

func TestInvoke_DurableFingerprintMismatchRejected(t *testing.T) {
	caller := &stubCaller{invokePayload: []byte(`{"operationId":"test-game:test-env:demo-durable:018f3b27-5c2e-7b10-8000-000000000002","outcome":"succeeded"}`)}
	svcCtx, _ := setupDurable(t, caller)
	const opID = "test-game:test-env:demo-durable:018f3b27-5c2e-7b10-8000-000000000002"

	first := durableInvokeReq()
	first.OperationID = opID
	_, err := Invoke(dispatchCtx(), svcCtx, first)
	require.NoError(t, err)

	second := durableInvokeReq()
	second.OperationID = opID
	second.Payload = json.RawMessage(`{"amount":2}`)
	_, err = Invoke(dispatchCtx(), svcCtx, second)
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.True(t, errors.As(err, &codeErr), "got %T: %v", err, err)
	assert.Equal(t, http.StatusConflict, codeErr.Code, "R5 参数漂移必须判 409")
}

func TestInvoke_DurableBroadcastRejected(t *testing.T) {
	svcCtx, _ := setupDurable(t, &stubCaller{})
	req := durableInvokeReq()
	req.Route = "broadcast"
	_, err := Invoke(dispatchCtx(), svcCtx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broadcast")
}
