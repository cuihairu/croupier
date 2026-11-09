package console

// 草稿预览执行补测：ExecuteDraftCompositeBinding 的 handler 层（绑定失败/
// 成功透传）、无 scope 上下文拒绝臂、executeBindingFromSnapshot 幂等链的
// Begin 失败/完成回写失败/已完成结果损坏三臂，以及 buildDraftBindingContracts
// （幽灵函数、多绑定排序）与 specDiagnosticDetails（空机器码回退）纯函数臂。
//
// 诚实跳过：
//   - ExecuteDraftCompositeBinding 的 buildDraftBindingContracts 错误透传臂
//     （294-295）：草稿绑定由生成链从同一批契约分配，bindings ⊆ contracts
//     恒成立，公开路径不可达幽灵函数；幽灵函数臂由本文件直接单测
//     buildDraftBindingContracts 覆盖（529）。
//   - completeExecuteIdempotency 的 marshal 失败臂（514-516）：入参是全类型化
//     的 spec.PageExecutionResult，json.Marshal 恒成功。

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

// handler 层：绑定失败 400（80-83）；成功请求透传 Idempotency-Key 后 200
// （85-91）。真实执行链由 service 层用例覆盖，这里验证端点接线。
func TestExecuteDraftCompositeBinding_HandlerArms(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"items":[],"total":0}`)},
	})
	handler := NewHandler(service)

	// 缺 required 字段 → 绑定失败 400
	badCtx, badRec := newGinContext(t, newGinRequest(http.MethodPost,
		"/api/v1/versioning/pages/composite/preview/execute", `{}`))
	handler.ExecuteDraftCompositeBinding(badCtx)
	assert.Equal(t, http.StatusBadRequest, badRec.Code)

	// service 错误（缺 function:invoke 权限）→ 统一错误映射
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

	// 合法草稿请求 → 200 + 同步执行结果
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

// 无 scope 上下文（有权限、有用户名）→ requireScope 拒绝（268）。
func TestExecuteDraftCompositeBinding_ScopeMissing(t *testing.T) {
	service, _ := newConsoleTestService(t, "function:invoke")
	scopeless := context.WithValue(context.Background(), "username", "console_tester")

	_, err := service.ExecuteDraftCompositeBinding(scopeless, draftTableRequest("draft--noscope", "player.list"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

// buildDraftBindingContracts：绑定引用不存在函数 → 校验错误（529）；
// 双绑定按 BindingID 排序（544 less 闭包）。
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

// specDiagnosticDetails：空机器码回退为 "diagnostic"（565）。
func TestSpecDiagnosticDetails_EmptyCode(t *testing.T) {
	details := specDiagnosticDetails([]spec.Diagnostic{{Message: "boom"}})
	require.Len(t, details, 1)
	assert.Equal(t, "boom", details["diagnostic"], "空机器码回退为 diagnostic 键")
}

// consoleDBFailpoint 按表内第 n 次指定操作注入确定性错误（幂等链路专用）。
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
		// Before + AddError：核心 Update 回调见 Error 即短路，写入不落地。
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, reg))
	case "create":
		require.NoError(t, db.Callback().Create().After("gorm:create").Register(name, reg))
	default:
		t.Fatalf("unknown failpoint op %q", fp.op)
	}
}

var errConsoleIdemFailpoint = errors.New("failpoint: idempotency store down")

// seededDraftExecuteService 建立契约 + 假执行器的草稿执行 fixture。
func seededDraftExecuteService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, upsertConsoleFunctionContract(service.svcCtx, ctx, draftInputSchema, draftOutputSchema, "safe", "player:query"))
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{
		caller: &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)},
	})
	return service, ctx
}

// 幂等链：Begin 占位写入失败 → 执行拒绝（486）。
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

// 幂等链：已完成行回写结果损坏 → 重放读取失败（499）。
func TestDraftExecuteIdempotency_StoredResultUnreadable(t *testing.T) {
	service, ctx := seededDraftExecuteService(t)
	req := draftTableRequest("draft--idem-replay", "player.list")
	req.IdempotencyKey = "idem-replay-1"

	// 首次执行成功并回写结果
	_, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.NoError(t, err)

	// 损坏已存储结果
	require.NoError(t, service.svcCtx.DB.Model(&model.PageExecuteIdempotency{}).
		Where("idempotency_key = ?", "idem-replay-1").
		Update("result", "not-json").Error)

	// 同 key 重放 → completed 行结果不可解析
	_, err = service.ExecuteDraftCompositeBinding(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page execute idempotency stored result unreadable")
}

// 幂等链：Complete 回写失败不阻断本次执行结果（518）。
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

	// 占位行保持 running（未 completed）
	var claim model.PageExecuteIdempotency
	require.NoError(t, service.svcCtx.DB.Where("idempotency_key = ?", "idem-complete-1").
		First(&claim).Error)
	assert.Equal(t, model.PageExecuteIdempotencyRunning, claim.Status)
}
