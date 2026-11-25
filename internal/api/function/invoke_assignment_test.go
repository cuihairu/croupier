package function

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assignment gate (OPEN-ISSUES #36 / BUG-032): when assignment records exist
// they act as an execution allowlist, and an unassigned function must be
// rejected with 403 function_not_assigned without reaching the agent (same
// position and rationale as the E2 disable intercept: a rejected call is not
// an execution). The default-open behavior when no records exist is
// implicitly regressed by the existing invoke cases (the fixture does not
// write an assignments file).
func writeInvokeAssignments(t *testing.T, f *invokeFixture, data string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assignments.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	f.svcCtx.Config.Registry.AssignmentsPath = path
}

// Red before the fix: with no gate, an unassigned function executes as usual
// (caller.requests non-empty).
func TestFunctionInvoke_UnassignedFunctionDenied(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	// The allowlist for scope demo|prod only holds demo.echo... let that pass
	// first, then add a second function to prove rejection
	writeInvokeAssignments(t, f, `{"demo|prod":["demo.echo"]}`)

	var codeErr *errorx.CodeError
	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.other", Payload: []byte(`{"x":1}`),
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
	// The request must not reach the agent
	assert.Empty(t, f.caller.requests)

	// async mode goes through the same gate
	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.other", Mode: "async", Payload: []byte(`{}`),
	})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
	assert.Empty(t, f.caller.requests)
}

// Functions in the list pass; scopes with no records default to open.
func TestFunctionInvoke_AssignedFunctionPasses(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	writeInvokeAssignments(t, f, `{"demo|prod":["demo.echo"]}`)

	resp, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.echo", Payload: []byte(`{"x":1}`),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"echo":true}`, string(resp.Result))
	require.Len(t, f.caller.requests, 1)
}
