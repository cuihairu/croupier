package function

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 分配闸门（OPEN-ISSUES #36 / BUG-032）：assignments 记录存在时是执行白名单，
// 未分配函数必须 403 function_not_assigned 且不触达 agent（与 E2 禁用拦截
// 同位同理由：被拒的调用不是一次执行）。无记录默认开放由既有 invoke 用例
// 隐式回归（fixture 不写 assignments 文件）。
func writeInvokeAssignments(t *testing.T, f *invokeFixture, data string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assignments.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	f.svcCtx.Config.Registry.AssignmentsPath = path
}

// 修复前红：闸门不存在时未分配函数照常执行（caller.requests 非空）。
func TestFunctionInvoke_UnassignedFunctionDenied(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.echo")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{"echo":true}`)}
	f.resolver.callers["agent-1"] = f.caller
	// scope demo|prod 的白名单只有 demo.echo……先放行；再建第二个函数证拒绝
	writeInvokeAssignments(t, f, `{"demo|prod":["demo.echo"]}`)

	var codeErr *errorx.CodeError
	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.other", Payload: []byte(`{"x":1}`),
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, 403, codeErr.Code)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
	// 请求不得到达 agent
	assert.Empty(t, f.caller.requests)

	// async 模式走同一闸门
	_, err = NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID: "demo.other", Mode: "async", Payload: []byte(`{}`),
	})
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "function_not_assigned", codeErr.ErrorCode())
	assert.Empty(t, f.caller.requests)
}

// 列表内函数放行；其他 scope 无记录默认开放。
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
