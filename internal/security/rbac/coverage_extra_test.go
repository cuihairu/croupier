package rbac

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCasbinFixture 在临时目录中写一对 model/policy 文件，返回路径。
func writeCasbinFixture(t *testing.T, model, policy string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.conf")
	policyPath := filepath.Join(dir, "policy.csv")
	if err := os.WriteFile(modelPath, []byte(model), 0o600); err != nil {
		t.Fatalf("write model: %v", err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return modelPath, policyPath
}

// TestCasbinPolicy_KeyMatch2ArityError 在 matcher 中以单参数调用 keyMatch2，
// 触发 casbin 内置 KeyMatch2Func 的 arity 校验错误，覆盖 Can() 中
// Enforce 返回错误后 continue 的分支。
//
// 注：NewCasbinPolicy 里注册的自定义 keyMatch2 闭包永远不会被调用 ——
// casbin v2.135.0 的 FunctionMap.AddFunction 使用 sync.Map.LoadOrStore，
// 而 "keyMatch2" 已被内置函数表预注册，自定义版本被静默丢弃。
func TestCasbinPolicy_KeyMatch2ArityError(t *testing.T) {
	modelPath, policyPath := writeCasbinFixture(t, `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = keyMatch2(r.obj, p.obj) && keyMatch2(r.obj) && r.sub == p.sub && (p.act == "*" || r.act == p.act)
`, "p, user:alice, /api/v1/roles, GET\n")

	cp, err := NewCasbinPolicy(modelPath, policyPath)
	if err != nil {
		t.Fatalf("NewCasbinPolicy: %v", err)
	}

	// 单参数 keyMatch2 使内置函数报 arity 错误 => Enforce 出错 => Can 应 DENY。
	if cp.Can("alice", "roles:read") {
		t.Error("expected DENY when matcher function reports arity error")
	}

	obj, act := cp.parsePermission("roles:read")
	if _, err := cp.enforcer.Enforce("user:alice", obj, act); err == nil {
		t.Error("expected arity error from single-arg keyMatch2 call")
	}
}

// TestCasbinPolicy_Can_EnforceError 用 4 个 request token 的模型触发
// casbin "invalid request size" 错误，覆盖 Can() 中 Enforce 出错 continue 的分支。
func TestCasbinPolicy_Can_EnforceError(t *testing.T) {
	modelPath, policyPath := writeCasbinFixture(t, `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`, "p, user:alice, /api/v1/roles, GET\n")

	cp, err := NewCasbinPolicy(modelPath, policyPath)
	if err != nil {
		t.Fatalf("NewCasbinPolicy: %v", err)
	}

	// Can 只传 3 个值，模型要求 4 个 => 两种 user 格式均 enforce 失败 => DENY。
	if cp.Can("alice", "roles:read") {
		t.Error("expected DENY when enforcer errors on request size mismatch")
	}
}

// TestEnhancedEvaluator_UnreachableViaEvaluateTerm 直接调用内部函数，
// 覆盖经 EvaluateAllowIf 不可达的兜底分支。
func TestEnhancedEvaluator_InternalFallbacks(t *testing.T) {
	evaluator := NewEnhancedEvaluator(NewPolicy())
	ctx := context.Background()
	authCtx := &AuthContext{User: "u1", Now: time.Now()}

	t.Run("evaluateFunction without paren", func(t *testing.T) {
		// evaluateTerm 已保证包含 "(" 才进入 evaluateFunction，
		// 直接调用无括号输入覆盖 openParen == -1 兜底。
		if evaluator.evaluateFunction(ctx, authCtx, "no_paren_term") {
			t.Error("term without paren must evaluate false")
		}
	})

	t.Run("evaluateComparison without operator", func(t *testing.T) {
		// containsComparisonOperator 已过滤含操作符的 term，
		// 直接调用无操作符输入覆盖循环结束后的 return false。
		if evaluator.evaluateComparison(ctx, authCtx, "plainTerm") {
			t.Error("term without comparison operator must evaluate false")
		}
	})

	t.Run("compareValues with unknown operator", func(t *testing.T) {
		if evaluator.compareValues(1.0, 2.0, "~=") {
			t.Error("unknown operator must evaluate false")
		}
	})
}

// TestEnhancedEvaluator_TimeWindowSplitMismatch 覆盖 evaluateTimeWindow：
// term 匹配时间段正则但按 "-" 切分后不是 2 段（且不匹配星期）时的 return false。
func TestEnhancedEvaluator_TimeWindowSplitMismatch(t *testing.T) {
	evaluator := NewEnhancedEvaluator(NewPolicy())
	authCtx := &AuthContext{User: "u1", Now: time.Now()}

	if evaluator.EvaluateAllowIf(context.Background(), authCtx, "09:00-17:00-x") {
		t.Error("time window with 3 dash parts and no day match must evaluate false")
	}
}

// TestUnifiedPolicyEngine_ValidateApprovals_Empty 覆盖 validateApprovals
// 在 approvals 为空时的快速失败分支。
func TestUnifiedPolicyEngine_ValidateApprovals_Empty(t *testing.T) {
	engine := NewUnifiedPolicyEngine(NewPolicy())

	if engine.validateApprovals(context.Background(), &TwoPersonRulePolicy{Threshold: 1}, nil, time.Now()) {
		t.Error("empty approvals must fail validation")
	}
}

// TestUnifiedPolicyEngine_ParseExpiryTime_Invalid 覆盖 parseExpiryTime
// 在 duration 解析为 0 时返回 nil 的分支。
func TestUnifiedPolicyEngine_ParseExpiryTime_Invalid(t *testing.T) {
	engine := NewUnifiedPolicyEngine(NewPolicy())

	if got := engine.parseExpiryTime("not-a-duration", time.Now()); got != nil {
		t.Errorf("invalid expiry time must yield nil, got %v", got)
	}
}
