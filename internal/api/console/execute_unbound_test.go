package console

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flipContractExecutionState 直改契约行执行状态：模拟上传管线 unbound 产物
// （CreateUnboundContract 只建新行，本测试在既有 bound 行上翻转，等价于
// unbound 物料 + 同名快照的执行场景）与 T6 自动绑定后的正向翻转。
func flipContractExecutionState(t *testing.T, service *Service, state spec.ExecutionState) {
	t.Helper()
	require.NoError(t, service.svcCtx.DB.Exec(
		"UPDATE function_contracts SET execution_state = ? WHERE function_id = ?",
		string(state), "player.query").Error)
}

// T8/D2 执行边界：契约 executionState=unbound（上传物料未绑定运行时执行器）
// 时以 409 executor_unbound 阻断（先于 freshness、晚于审计 defer——阻断也
// 留审计）；翻转为 bound（T6 自动绑定的运行时效果）后执行照常放行。
func TestExecuteBindingBlockedWhenExecutorUnbound(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	flipContractExecutionState(t, service, spec.ExecutionStateUnbound)
	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	})

	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "unbound 执行边界必须返回结构化 CodeError")
	assert.Equal(t, "executor_unbound", codeErr.ErrorCode())
	status, body := codeErr.Data()
	assert.Equal(t, http.StatusConflict, status)
	bodyMap, ok := body.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "executor_unbound", bodyMap["error"])
	assert.Empty(t, caller.lastRequest, "阻断先于 payload 组装，不得触达执行器")

	// 阻断也写审计（检查位于 audit defer 之后）：failure 留痕可追溯
	records, total, listErr := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageExecute},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, listErr)
	require.Equal(t, 1, total)
	assert.Equal(t, "failure", records[0].Outcome)
	assert.Equal(t, "player.query", records[0].Details["function_id"])

	// bound 翻转后不受影响：同名注册（或绑定）翻转 bound → 执行照常放行
	flipContractExecutionState(t, service, spec.ExecutionStateBound)
	resp, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"alice"}`),
		},
	})

	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindSync, resp.Result.Kind)
	assert.JSONEq(t, `{"ok":true}`, string(resp.Result.Data))
}

// handler 层：unbound 阻断经统一错误映射落 409 + error=executor_unbound
// （前端按 HTTP 409 分支、按 error 稳定码渲染「未绑定执行器」空态）。
func TestExecuteBindingHandlerSurfacesExecutorUnbound(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)},
	})
	flipContractExecutionState(t, service, spec.ExecutionStateUnbound)

	ginCtx, rec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/console/pages/player.manage/bindings/player.query/execute",
		`{"context":{"form":{"keyword":"alice"}}}`))
	ginCtx.Params = executeBindingURIParams("player.manage", "player.query")
	NewHandler(service).ExecuteBinding(ginCtx)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"executor_unbound"`)
	assert.Contains(t, rec.Body.String(), `"bindingId"`)
}
