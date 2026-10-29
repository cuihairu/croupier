package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── ARCH-025：策略查询/初始化故障禁止静默放行 ─────────────────────────
// 旧行为：GetPolicy 返回错误（function_policies 表缺失 / SQL 失败）时
// EnforceFunctionPolicy 返回 (nil, nil)，调用方把 functionPolicy 当「无政策」
// 继续执行——高危函数在无有效政策时绕过审批/审计直接派发。修复后：
// 策略存储/配置故障 = fail-closed（返回错误 → 拒绝执行），
// 「无覆盖策略」仍由 GetPolicy 回落到风险级默认策略（非 nil），二者区分开。

// 存储故障（表缺失）必须 fail-closed：返回错误而非 (nil, nil)。
func TestEnforceFunctionPolicy_StorageFailureFailsClosed(t *testing.T) {
	f := newInvokeFixture(t)
	require.NoError(t, f.db.Migrator().DropTable("function_policies"))

	_, err := enforceFunctionPolicy(context.Background(), f.svcCtx, "any.fn", "", "", []string{"admin"})
	require.Error(t, err, "GetPolicy 存储故障必须返回错误（fail-closed）")
}

// PolicyManager 初始化失败（nil 管理器）必须 fail-closed。
func TestEnforceFunctionPolicy_NilManagerFailsClosed(t *testing.T) {
	f := newInvokeFixture(t)
	f.svcCtx.PolicyManager = nil

	_, err := enforceFunctionPolicy(context.Background(), f.svcCtx, "any.fn", "", "", []string{"admin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy manager unavailable")
}

// 高风险函数 + 策略存储故障：Invoke 拒绝，且不派发（执行器零调用）。
func TestFunctionInvoke_PolicyStorageFailureRefusesHighRisk(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "risk.high")
	// 契约 x-risk=high → resolvePolicyRisk 命中 high。
	require.NoError(t, f.store.UpsertOpenAPI("risk.high", openAPIWithRisk(t, "high")))
	caller := &fakeSessionCaller{invokePayload: []byte(`{"ok":1}`)}
	f.resolver.callers["agent-1"] = caller
	require.NoError(t, f.db.Migrator().DropTable("function_policies"))

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{ID: "risk.high"})
	require.Error(t, err, "高风险函数在策略存储故障时必须被拒绝")
	assert.Len(t, caller.requests, 0, "被拒绝的执行不得派发到 agent")
}

// PolicyManager 初始化失败（nil）：Invoke 拒绝（fail-closed），不派发。
func TestFunctionInvoke_NilPolicyManagerRefuses(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "any.fn")
	caller := &fakeSessionCaller{invokePayload: []byte(`{"ok":1}`)}
	f.resolver.callers["agent-1"] = caller
	f.svcCtx.PolicyManager = nil

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{ID: "any.fn"})
	require.Error(t, err, "PolicyManager 初始化失败必须拒绝执行（fail-closed）")
	assert.Contains(t, err.Error(), "policy manager unavailable")
	assert.Len(t, caller.requests, 0)
}

// 边界：已批准续行（approved continuation）仍豁免 nil 管理器——上游已把关，
// 不因策略存储故障二次拒绝已获批准的调用。
func TestFunctionInvoke_ApprovedContinuationExemptFromNilManager(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "cont.fn")
	caller := &fakeSessionCaller{invokePayload: []byte(`{"ok":1}`)}
	f.resolver.callers["agent-1"] = caller
	f.svcCtx.PolicyManager = nil

	// 带 approved continuation 元数据（approvalBypass=approved）：不查
	// PolicyManager，直接执行。
	ctx := svc.WithGameScope(context.WithValue(context.Background(), "username", "opuser"),
		svc.GameScope{GameID: "demo", Env: "prod"})
	resp, err := NewService(f.svcCtx).FunctionInvoke(ctx, &FunctionInvokeRequest{
		ID:       "cont.fn",
		Metadata: map[string]string{"approvalBypass": "approved"},
	})
	require.NoError(t, err, "已批准续行不因 nil 管理器被拒绝")
	require.NotNil(t, resp)
}

func openAPIWithRisk(t *testing.T, risk string) *openapi3.Operation {
	t.Helper()
	op := openapi3.NewOperation()
	op.Extensions = map[string]interface{}{"x-risk": risk}
	return op
}
