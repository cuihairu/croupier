// 覆盖目标：页面绑定执行的服务端幂等（D4 安全重试）——Idempotency-Key 头
// 使同一执行意图在占位窗口内只真实下发一次：已完成重放存储结果、进行中
// 判结果未知 409、键误复用 409；无 key 时行为与既有无幂等链路完全一致。
// 覆盖验收面：成功、待审批、失败（结果未知）、超时后成功（重试拿到落地
// 结果）、重复提交；以及重放不重复写审计与执行留痕（一次动作一条执行记录）。
package console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
)

const idemInputSchema = `{"type":"object","properties":{"keyword":{"type":"string"}}}`
const idemOutputSchema = `{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"number"}}}`

// countingConsoleCaller 计数调用次数：断言「重放不重复下发副作用」。
type countingConsoleCaller struct {
	payload []byte
	err     error
	calls   int
}

func (c *countingConsoleCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	c.calls++
	inner := fakeConsoleSessionCaller{payload: c.payload, err: c.err}
	return inner.Call(ctx, msgID, reqBody)
}

// idemContext 构造同一逻辑动作的渲染器上下文（同 key 下字节必须一致）。
func idemContext(keyword string) ConsoleBindingExecutionContext {
	raw, _ := json.Marshal(map[string]string{"keyword": keyword})
	return ConsoleBindingExecutionContext{Form: json.RawMessage(raw)}
}

func TestExecuteBindingIdempotency_SuccessThenReplay(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	w := attachExecutionLogWriter(t, service.svcCtx)
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, idemInputSchema, idemOutputSchema))
	caller := &countingConsoleCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	req := &ConsoleExecuteBindingRequest{
		PageKey:        "player.manage",
		BindingID:      "player.query",
		Context:        idemContext("k1"),
		IdempotencyKey: "retry-1",
	}
	first, err := service.ExecuteBinding(ctx, req)
	require.NoError(t, err)
	assert.False(t, first.Replayed, "首次执行不是重放")
	assert.Equal(t, spec.PageExecutionKindSync, first.Result.Kind)
	require.NotEmpty(t, first.Result.RequestID)
	assert.Equal(t, 1, caller.calls)

	// 重复提交（同 key 同上下文）：重放存储结果，不重复下发、不重复写留痕。
	second, err := service.ExecuteBinding(ctx, req)
	require.NoError(t, err)
	assert.True(t, second.Replayed, "同 key 重复提交应重放")
	assert.Equal(t, first.Result.RequestID, second.Result.RequestID, "重放返回同一请求 id")
	assert.JSONEq(t, string(first.Result.Data), string(second.Result.Data))
	assert.Equal(t, 1, caller.calls, "重放不得再次下发执行")

	// 一次动作一条审计记录、一条执行留痕。
	records, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventPageExecute}}, audit.AuditPage{PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, 1, total, "重放不得重复写审计链")
	require.Len(t, records, 1)
	assert.Equal(t, "success", records[0].Outcome)

	w.Stop()
	var logs []model.ExecutionLog
	require.NoError(t, service.svcCtx.DB.Find(&logs).Error)
	assert.Len(t, logs, 1, "重放不得重复写执行留痕")

	// 占位行已回写 completed 与结果 JSON。
	rec, err := model.NewPageExecuteIdempotencyModel(service.svcCtx.DB).
		Find(ctx, "demo-game", "development", "player.manage", "player.query", "retry-1")
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, model.PageExecuteIdempotencyCompleted, rec.Status)
	assert.Equal(t, "console_tester", rec.Actor)
	assert.Equal(t, "player.query", rec.FunctionID)
	assert.NotEmpty(t, rec.RequestHash)
	var stored spec.PageExecutionResult
	require.NoError(t, json.Unmarshal(rec.Result, &stored))
	assert.Equal(t, first.Result.RequestID, stored.RequestID)
}

// TestExecuteBindingIdempotency_NoKeyUnchanged 无 key 时行为与既有完全一致：
// 两次同上下文提交都真实下发（幂等不启用、不阻断正常重复执行）。
func TestExecuteBindingIdempotency_NoKeyUnchanged(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, idemInputSchema, idemOutputSchema))
	caller := &countingConsoleCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	req := &ConsoleExecuteBindingRequest{PageKey: "player.manage", BindingID: "player.query", Context: idemContext("k1")}
	first, err := service.ExecuteBinding(ctx, req)
	require.NoError(t, err)
	assert.False(t, first.Replayed)
	second, err := service.ExecuteBinding(ctx, req)
	require.NoError(t, err)
	assert.False(t, second.Replayed, "无 key 不启用幂等")
	assert.NotEqual(t, first.Result.RequestID, second.Result.RequestID, "两次独立执行各自产生请求 id")
	assert.Equal(t, 2, caller.calls, "无 key 时重复提交仍真实下发")

	var rows int64
	require.NoError(t, service.svcCtx.DB.Model(&model.PageExecuteIdempotency{}).Count(&rows).Error)
	assert.Zero(t, rows, "无 key 时不得写入幂等表")
}

// TestExecuteBindingIdempotency_DifferentPayloadRejected 同 key 但请求内容
// 不同 = 键误复用（409 idempotency_key_reused）：不重放、不占位，换 key 可
// 正常执行。
func TestExecuteBindingIdempotency_DifferentPayloadRejected(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, idemInputSchema, idemOutputSchema))
	caller := &countingConsoleCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	base := &ConsoleExecuteBindingRequest{PageKey: "player.manage", BindingID: "player.query", Context: idemContext("k1"), IdempotencyKey: "dup-1"}
	_, err := service.ExecuteBinding(ctx, base)
	require.NoError(t, err)
	assert.Equal(t, 1, caller.calls)

	other := &ConsoleExecuteBindingRequest{PageKey: "player.manage", BindingID: "player.query", Context: idemContext("k2"), IdempotencyKey: "dup-1"}
	_, err = service.ExecuteBinding(ctx, other)
	require.Error(t, err)
	assert.Equal(t, "idempotency_key_reused", err.(*errorx.CodeError).ErrorCode())
	assert.Equal(t, 1, caller.calls, "键误复用不得下发执行")

	// 新 key 可正常执行（前一次占位未污染后续动作）。
	next := &ConsoleExecuteBindingRequest{PageKey: "player.manage", BindingID: "player.query", Context: idemContext("k2"), IdempotencyKey: "dup-2"}
	third, err := service.ExecuteBinding(ctx, next)
	require.NoError(t, err)
	assert.False(t, third.Replayed)
	assert.Equal(t, 2, caller.calls)
}

// TestExecuteBindingIdempotency_InUseAfterFailedDispatch 下发失败（超时/中断）
// 时结果未知：占位保持 running，同 key 重试判 409 idempotency_key_in_use，
// 提示用同一请求稍后重试而非换新 key 盲目重跑。
func TestExecuteBindingIdempotency_InUseAfterFailedDispatch(t *testing.T) {
	service, ctx, auditStore := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, idemInputSchema, idemOutputSchema))
	caller := &countingConsoleCaller{err: assert.AnError}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	req := &ConsoleExecuteBindingRequest{PageKey: "player.manage", BindingID: "player.query", Context: idemContext("k1"), IdempotencyKey: "unknown-1"}
	_, err := service.ExecuteBinding(ctx, req)
	require.Error(t, err)
	assert.Equal(t, 1, caller.calls)

	_, err = service.ExecuteBinding(ctx, req)
	require.Error(t, err)
	assert.Equal(t, "idempotency_key_in_use", err.(*errorx.CodeError).ErrorCode())
	assert.Equal(t, 1, caller.calls, "结果未知期间不得再次下发")

	// 两次尝试各记一条审计（被拒也是请求，与门禁失败一致），但只有第一次
	// 真实下发过副作用。
	records, total, err := auditStore.List(audit.AuditFilter{EventType: []audit.AuditEventType{audit.EventPageExecute}}, audit.AuditPage{PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total, "两次提交各留一条审计，均不重复下发")
	require.Len(t, records, 2)
	for _, record := range records {
		assert.Equal(t, "failure", record.Outcome)
	}
}

// TestDraftExecuteBindingIdempotency_ApprovalOnce 审批闸同样受幂等保护：
// 同 key 重复提交重放同一审批单号，不重复落审批单（重复提交不再制造重复工单）。
func TestDraftExecuteBindingIdempotency_ApprovalOnce(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	store := approvals.NewMemStore()
	service.svcCtx.ApprovalsStore = store
	require.NoError(t, seedDraftApprovingContract(service.svcCtx, ctx))

	req := draftTableRequest("draft--idem-approval", "player.list")
	req.IdempotencyKey = "approval-1"
	first, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.NoError(t, err)
	assert.False(t, first.Replayed)
	assert.Equal(t, spec.PageExecutionKindApproval, first.Result.Kind)
	require.NotEmpty(t, first.Result.ApprovalID)

	second, err := service.ExecuteDraftCompositeBinding(ctx, req)
	require.NoError(t, err)
	assert.True(t, second.Replayed)
	assert.Equal(t, first.Result.ApprovalID, second.Result.ApprovalID, "重放返回同一审批单号")
	assert.Equal(t, first.Result.RequestID, second.Result.RequestID)

	stored, total, err := store.List(approvals.Filter{}, approvals.Page{Page: 1, Size: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total, "重复提交不得创建第二条审批单")
	assert.Len(t, stored, 1)
}

// TestExecuteBindingHandlerIdempotencyHeader 覆盖请求头接线：Idempotency-Key
// 经 handler 归一后进入服务端幂等；非法头按未携带处理，不阻断执行。
func TestExecuteBindingHandlerIdempotencyHeader(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, idemInputSchema, idemOutputSchema))
	caller := &countingConsoleCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})
	handler := NewHandler(service)

	const execBody = `{"context":{"form":{"keyword":"alice"}}}`
	execute := func(idemHeader string) *httptest.ResponseRecorder {
		req := newGinRequest(http.MethodPost,
			"/api/v1/console/pages/player.manage/bindings/player.query/execute", execBody)
		if idemHeader != "" {
			req.Header.Set("Idempotency-Key", idemHeader)
		}
		ginCtx, rec := newGinContext(t, req)
		ginCtx.Params = executeBindingURIParams("player.manage", "player.query")
		handler.ExecuteBinding(ginCtx)
		return rec
	}

	first := execute("hdr-1")
	require.Equal(t, http.StatusOK, first.Code)
	assert.NotContains(t, first.Body.String(), `"replayed"`)

	second := execute("hdr-1")
	require.Equal(t, http.StatusOK, second.Code)
	assert.Contains(t, second.Body.String(), `"replayed":true`, "同 key 头重放存储结果")
	assert.Equal(t, 1, caller.calls, "同 key 不得重复下发副作用")

	// 非法头（含空格与符号）按未携带处理：正常执行、不启用幂等。
	bad := execute("bad key!@#")
	require.Equal(t, http.StatusOK, bad.Code)
	assert.NotContains(t, bad.Body.String(), `"replayed"`)
	assert.Equal(t, 2, caller.calls)
}

// TestNormalizeIdempotencyKey 覆盖请求头归一契约：合法字符集 [A-Za-z0-9._:-]、
// 上限 128；trim 两侧空白；任何非法形态按未携带处理（空串），坏头不阻断执行。
func TestNormalizeIdempotencyKey(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"blank", "   ", ""},
		{"trimmed", "  retry-7  ", "retry-7"},
		{"allowed charset", "abc-DEF_123.zzz:ok", "abc-DEF_123.zzz:ok"},
		{"space inside", "bad space", ""},
		{"slash", "bad/slash", ""},
		{"unicode", "键-1", ""},
		{"over long", strings.Repeat("a", idempotencyKeyMaxLen+1), ""},
		{"at limit", strings.Repeat("a", idempotencyKeyMaxLen), strings.Repeat("a", idempotencyKeyMaxLen)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeIdempotencyKey(tc.raw))
		})
	}
}
