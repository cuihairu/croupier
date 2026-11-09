package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/platform/dispatch"
	functioninvoke "github.com/cuihairu/croupier/internal/platform/functioninvoke"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

func policyRiskFromContract(risk string) policy.RiskLevel {
	return functioninvoke.PolicyRiskFromContract(risk)
}

func resolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) (policy.RiskLevel, error) {
	return functioninvoke.ResolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)
}

// ResolvePolicyRisk exposes resolvePolicyRisk to the approve path
// (internal/api/approval step-up gate, OPEN-ISSUES #75): 高危审批门槛必须与
// 调用路径共用同一风险判定口径，禁止各包私抄解析逻辑。来源故障向上传播，
// 调用侧按 fail-closed 处理（step-up 门槛视作高危）。
func ResolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) (policy.RiskLevel, error) {
	return functioninvoke.ResolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)
}

func enforceFunctionPolicy(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string, userRoles []string) (*policy.Policy, error) {
	return functioninvoke.EnforceFunctionPolicy(ctx, svcCtx, functionID, gameID, env, userRoles)
}

func matchAnyRole(userRoles []string, allowedRoles []string) bool {
	return functioninvoke.MatchAnyRole(userRoles, allowedRoles)
}

func buildBroadcastResponse(b *dispatch.BroadcastInvocation) *FunctionInvokeResponse {
	return functionInvokeResponseFromUnified(functioninvoke.BuildBroadcastResponse(b))
}
