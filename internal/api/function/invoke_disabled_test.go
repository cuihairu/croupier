package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2 disable intercept: the dashboard disable button writes functions.status,
// and functionInvoke must read it back and reject disabled functions (409
// function_disabled), with sync and async sharing the same gate.
func TestFunctionInvoke_DisabledFunctionBlocked(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	// Materialize the disabled row (simulating the result of
	// setFunctionEnabled(fn, StatusDisabled))
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
	// The request must not reach the agent
	assert.Empty(t, f.caller.requests)

	// async mode goes through the same gate (the gate sits before Mode branching)
	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Mode: "async", Payload: []byte(`{}`),
	})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_disabled", codeErr.ErrorCode())
	assert.Empty(t, f.caller.requests)
}

// After re-enabling the gate lets the call through (also covering the
// "disable write path -> execution read path" loop).
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
