package function

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// validateInvokeInput：dispatch 前按 descriptor inputSchema 的 required
// 校验 payload——缺参数是客户端错误（400 + details 指明字段），不得透传
// agent 后以 5xx 路由错误收场。

func upsertInputContract(t *testing.T, f *invokeFixture, functionID string, input datatypes.JSONMap) {
	t.Helper()
	require.NoError(t, f.svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: functionID,
		Version:    "v1",
		Input:      input,
	}))
}

func TestValidateInvokeInput_MissingRequired(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId", "amount"},
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
			"amount":   map[string]interface{}{"type": "integer"},
		},
	})

	err := validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":"p-1"}`))
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Message, "amount")
	assert.NotContains(t, codeErr.Message, "playerId")
	assert.Equal(t, "required", codeErr.Details["amount"])
	assert.NotContains(t, codeErr.Details, "playerId")
}

func TestValidateInvokeInput_ProvidedPasses(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId"},
	})

	// 空串/false 是 JSON Schema 合法值（required 只约束存在性）。
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":""}`)))
	// 显式 null 视为缺失。
	require.Error(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":null}`)))
	// 空对象 payload。
	require.Error(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{}`)))
}

func TestValidateInvokeInput_NoContractPassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	// 无 descriptor：透传（契约由游戏服兜底）。
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.bare"}, []byte(`{}`)))
	// descriptor 有 Input 但无 required：透传。
	upsertInputContract(t, f, "demo.noreq", datatypes.JSONMap{
		"type": "object",
		"properties": map[string]interface{}{
			"x": map[string]interface{}{"type": "string"},
		},
	})
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.noreq"}, []byte(`{}`)))
	// 非对象 payload（数组）不在此层拦截。
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"required": []interface{}{"playerId"},
	})
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`[1,2]`)))
	// FunctionModel 未初始化：跳过。
	require.NoError(t, validateInvokeInput(context.Background(), nil,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{}`)))
}

// 缺参必须在 dispatch 前被拒：agent 在线也不应收到请求（否则用户看到的
// 是 no live agent 之类的路由错误而不是缺了哪个参数）。
func TestFunctionInvoke_MissingRequiredRejectedBeforeDispatch(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.grant")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{}`)}
	f.resolver.callers["agent-1"] = f.caller
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId", "amount"},
	})

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID:      "demo.grant",
		Payload: []byte(`{}`),
	})
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Message, "playerId")
	assert.Contains(t, codeErr.Message, "amount")
	assert.Empty(t, f.caller.requests)
}
