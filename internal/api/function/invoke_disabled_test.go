package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2 禁用拦截：dashboard 禁用按钮写 functions.status，functionInvoke 必须
// 回读并拒绝禁用函数（409 function_disabled），同步/异步同一闸门。
func TestFunctionInvoke_DisabledFunctionBlocked(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	// 物化禁用行（模拟 setFunctionEnabled(fn, StatusDisabled) 的结果）
	require.NoError(t, f.svcCtx.FunctionModel.Create(context.Background(), &model.Function{
		FunctionID: "demo.echo",
		GameID:     "demo",
		Status:     model.StatusDisabled,
	}))

	var codeErr *errorx.CodeError
	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{"x":1}`),
	})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 409, codeErr.Code)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	// 请求不得到达 agent
	assert.Empty(t, f.caller.requests)

	// async 模式走同一闸门（闸门在 Mode 分流之前）
	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Mode: "async", Payload: []byte(`{}`),
	})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	assert.Empty(t, f.caller.requests)
}

// 重新启用后闸门放行（同时覆盖「禁用写路径 → 执行读路径」闭环）。
func TestFunctionInvoke_ReEnabledFunctionPasses(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller

	fn := &model.Function{FunctionID: "demo.echo", GameID: "demo", Status: model.StatusDisabled}
	require.NoError(t, f.svcCtx.FunctionModel.Create(context.Background(), fn))
	require.NoError(t, setFunctionEnabled(context.Background(), f.svcCtx, "demo.echo", model.StatusEnabled))

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{"x":1}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"echo":true}`, string(resp.Result))
	require.Len(t, f.caller.requests, 1)
}
