package console

// Additional coverage for draft preview execution: the handler layer of
// ExecuteDraftCompositeBinding (binding failure / success pass-through), the
// rejected arm of a scope-less context, the three arms of the
// executeBindingFromSnapshot idempotency chain (Begin failure / Complete
// write-back failure / unreadable completed result), and the pure-function arms
// of buildDraftBindingContracts (ghost function, multi-binding ordering) and
// specDiagnosticDetails (empty machine code fallback).
//
// Honest skips:
//   - The buildDraftBindingContracts error pass-through arm of
//     ExecuteDraftCompositeBinding (294-295): draft bindings are allocated from
//     the same contract batch by the generation chain, so bindings ⊆ contracts
//     always holds and no public path reaches a ghost function; the ghost function
//     arm is covered here by unit-testing buildDraftBindingContracts directly
//     (529).
//   - The marshal failure arm of completeExecuteIdempotency (514-516): the input
//     is a fully typed spec.PageExecutionResult, so json.Marshal always succeeds.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Handler layer: binding failure returns 400 (80-83); a successful request
// passes the Idempotency-Key through and returns 200 (85-91). The real execution
// chain is covered by service-layer cases; this one verifies the endpoint wiring.
func TestExecuteDraftCompositeBinding_HandlerArms(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"items":[],"total":0}`)},
	})
	handler := NewHandler(service)

	// A missing required field -> binding failure 400
	badCtx, badRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/versioning/pages/composite/preview/execute", `{}`))
	handler.ExecuteDraftCompositeBinding(badCtx)
	assert.Equal(t, http.StatusBadRequest, badRec.Code)

	// A service error (missing function:invoke permission) -> the unified error mapping
	denied, dCtx := newConsoleTestService(t, "console:read")
	require.NoError(t, upsertConsoleFunctionContract(denied.svcCtx, dCtx, draftInputSchema, draftOutputSchema, "safe", "player:query"))
	denyCtx, denyRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/versioning/pages/composite/preview/execute",
		`{"pageKey":"draft--deny","bindingId":"player.list",`+
			`"sections":[{"key":"player.list","functionId":"player.query","view":"table"}],`+
			`"context":{"form":{"keyword":"alice"}}}`))
	NewHandler(denied).ExecuteDraftCompositeBinding(denyCtx)
	assert.Equal(t, http.StatusForbidden, denyRec.Code)
	assert.Contains(t, denyRec.Body.String(), "无权执行运行控制台操作")

	// A well-formed draft request -> 200 plus the synchronous execution result
	ginCtx, rec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/versioning/pages/composite/preview/execute",
		`{"pageKey":"draft--handler","bindingId":"player.list",`+
			`"sections":[{"key":"player.list","functionId":"player.query","view":"table"}],`+
			`"context":{"form":{"keyword":"alice"}}}`))
	ginCtx.Request.Header.Set("Idempotency-Key", "handler-idem-1")
	handler.ExecuteDraftCompositeBinding(ginCtx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"kind":"sync"`)
}

// A scope-less context (permission and username present) -> rejected by requireScope (268).
func TestExecuteDraftCompositeBinding_ScopeMissing(t *testing.T) {
	service, _ := newConsoleTestService(t, "function:invoke")
	scopeless := context.WithValue(context.Background(), "username", "console_tester")

	_, err := service.ExecuteDraftCompositeBinding(scopeless, draftTableRequest("draft--noscope", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

// buildDraftBindingContracts: a binding referencing a non-existent function ->
// validation error (529); two bindings sorted by BindingID (544 less closure).
func TestBuildDraftBindingContracts_Arms(t *testing.T) {
	functions := map[string]spec.FunctionSpec{
		"player.query": {ID: "player.query", Version: "1.0.0"},
	}

	_, err := buildDraftBindingContracts([]spec.PageFunctionBinding{
		{ID: "b", FunctionID: "player.query"},
		{ID: "a", FunctionID: "ghost.fn"},
	}, functions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bound function does not exist: ghost.fn")

	snapshots, err := buildDraftBindingContracts([]spec.PageFunctionBinding{
		{ID: "b2", FunctionID: "player.query"},
		{ID: "a2", FunctionID: "player.query"},
	}, functions)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	assert.Less(t, snapshots[0].BindingID, snapshots[1].BindingID, "快照按 BindingID 排序")
}

// specDiagnosticDetails: an empty machine code falls back to "diagnostic" (565).
func TestSpecDiagnosticDetails_EmptyCode(t *testing.T) {
	details := specDiagnosticDetails([]spec.Diagnostic{{Message: "boom"}})
	require.Len(t, details, 1)
	assert.Equal(t, "boom", details["diagnostic"], "空机器码回退为 diagnostic 键")
}

// consoleDBFailpoint injects a deterministic error on the nth occurrence of a
// given operation against a table (dedicated to the idempotency chain).
type consoleDBFailpoint struct {
	table string
	op    string // query / update / create
	n     int
	err   error
}

func applyConsoleFailpoint(t *testing.T, db *gorm.DB, fp consoleDBFailpoint) {
	t.Helper()
	var hits int64
	reg := func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != fp.table {
			return
		}
		hits++
		if hits == int64(fp.n) {
			_ = tx.AddError(fp.err)
		}
	}
	name := fmt.Sprintf("test:console_idem_%s_%s", fp.table, fp.op)
	switch fp.op {
	case "query":
		require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, reg))
	case "update":
		// Before + AddError: the core Update callback short-circuits as soon as it sees
		// an Error, so the write never lands.
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().After("gorm:create").Register(name, reg))
	default:
		t.Fatalf("unknown failpoint op %q", fp.op)
	}
}

var errConsoleIdemFailpoint = errors.New("failpoint: idempotency store down")

// seededDraftExecuteService builds a draft execution fixture with a contract and
// a fake executor.
func seededDraftExecuteService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)},
	})
	return service, ctx
}

// Idempotency chain: a failed Begin placeholder write -> execution rejected (486).
func TestDraftExecuteIdempotency_BeginError(t *testing.T) {
	service, ctx := seededDraftExecuteService(t)
	req := draftTableRequest("draft--idem-begin", "player.list")
	req.IdempotencyKey = "idem-begin-1"
	applyConsoleFailpoint(t, service.svcCtx.DB, consoleDBFailpoint{
		table: "page_execute_idempotencies", op: "create", n: 1, err: errConsoleIdemFailpoint,
	})

	_, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.ErrorIs(t, err, errConsoleIdemFailpoint)
	assert.Contains(t, err.Error(), "page execute idempotency claim")
}

// Idempotency chain: a corrupted result written back to the completed row ->
// replay read fails (499).
func TestDraftExecuteIdempotency_StoredResultUnreadable(t *testing.T) {
	service, ctx := seededDraftExecuteService(t)
	req := draftTableRequest("draft--idem-replay", "player.list")
	req.IdempotencyKey = "idem-replay-1"

	// The first execution succeeds and writes back the result
	_, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.NoError(t, err)

	// Corrupt the stored result
	require.NoError(t, service.svcCtx.DB.Model(&model.PageExecuteIdempotency{}).
		Where("idempotency_key = ?", "idem-replay-1").
		Update("result", "not-json").Error)

	// Replaying the same key -> the completed row's result is unreadable
	_, err = service.ExecuteDraftCompositeBinding(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page execute idempotency stored result unreadable")
}

// Idempotency chain: a failed Complete write-back does not block the current
// execution result (518).
func TestDraftExecuteIdempotency_CompleteError(t *testing.T) {
	service, ctx := seededDraftExecuteService(t)
	req := draftTableRequest("draft--idem-complete", "player.list")
	req.IdempotencyKey = "idem-complete-1"
	applyConsoleFailpoint(t, service.svcCtx.DB, consoleDBFailpoint{
		table: "page_execute_idempotencies", op: "update", n: 1, err: errConsoleIdemFailpoint,
	})

	resp, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.NoError(t, err, "回写失败只影响后续重放，不得阻断本次执行")
	require.NotNil(t, resp.Result)

	// The placeholder row stays running (not completed)
	var claim model.PageExecuteIdempotency
	require.NoError(t, service.svcCtx.DB.Where("idempotency_key = ?", "idem-complete-1").
		First(&claim).Error)
	assert.Equal(t, model.PageExecuteIdempotencyRunning, claim.Status)
}
