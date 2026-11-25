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
// (internal/api/approval step-up gate, OPEN-ISSUES #75): the high-risk approval
// bar must share the same risk determination criteria as the invocation path;
// no package may privately re-implement the resolution logic. Source failures
// propagate upward and callers treat them fail-closed (the step-up bar counts
// as high-risk).
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
