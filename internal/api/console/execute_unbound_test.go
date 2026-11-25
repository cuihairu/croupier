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

// flipContractExecutionState flips the execution state of a contract row directly:
// it stands in for the unbound artifact of the upload pipeline (CreateUnboundContract
// only inserts a fresh row, so this test flips an existing bound row, which is
// equivalent to executing unbound material with a same-named snapshot) and for the
// forward flip caused by T6 automatic binding.
func flipContractExecutionState(t *testing.T, service *Service, state spec.ExecutionState) {
	t.Helper()
	require.NoError(t, service.svcCtx.DB.Exec(
		"UPDATE function_contracts SET execution_state = ? WHERE function_id = ?",
		string(state), "player.query").Error)
}

// T8/D2 execution boundary: when a contract's executionState=unbound (the uploaded
// material is not yet bound to a runtime executor) the call is blocked with 409
// executor_unbound (before freshness, after the audit defer -- a blocked call also
// leaves an audit trail); after flipping to bound (the runtime effect of T6 automatic
// binding) execution passes as usual.
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

	// A blocked call also writes an audit record (checked after the audit defer): the
	// failure is traceable
	records, total, listErr := auditStore.List(audit.AuditFilter{
		EventType: []audit.AuditEventType{audit.EventPageExecute},
	}, audit.AuditPage{PageSize: 10})
	require.NoError(t, listErr)
	require.Equal(t, 1, total)
	assert.Equal(t, "failure", records[0].Outcome)
	assert.Equal(t, "player.query", records[0].Details["function_id"])

	// Flipping to bound leaves execution unaffected: a same-named registration (or
	// binding) flips bound -> execution passes as usual
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

// Handler layer: the unbound block lands as 409 + error=executor_unbound via the
// unified error mapping (the frontend branches on HTTP 409 and renders the
// "executor not bound" empty state from the stable error code).
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
