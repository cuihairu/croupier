package functioninvoke

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

// PolicyRiskFromContract maps a contract risk word (safe/warning/high/danger)
// into the policy engine's risk vocabulary (low/medium/high/danger). The two
// vocabularies intentionally differ; this is the single seam. Unknown/empty
// values fall back to medium, preserving the historical default.
func PolicyRiskFromContract(risk string) policy.RiskLevel {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case string(spec.RiskSafe):
		return policy.RiskLow
	case string(spec.RiskWarning):
		return policy.RiskMedium
	case string(spec.RiskHigh):
		return policy.RiskHigh
	case string(spec.RiskDanger):
		return policy.RiskDanger
	default:
		return policy.RiskMedium
	}
}

// resolvePolicyRisk 解析函数治理风险，优先级：
//  1. 内存 openapiOperations 的 x-risk（converter 写入的键是 x-risk，此前误读
//     x-risk-level 永不命中）——本次启动内的显式声明，含 safe。
//  2. 契约行的非 safe risk——契约行是 OpenAPI 上传（x-risk）与注册物化的汇聚点，
//     重启后仍可解析；但 risk 列零值即 safe，无法区分「声明 safe」与「未声明」，
//     故契约行的 safe 不采信，未声明函数保守维持 medium（治理只会收紧不会放松）。
//  3. 皆无 → medium（历史默认）。
//
// 服务端治理覆盖入口是 PolicyManager 的 per-function DB override（SetOverride），
// GetPolicy 优先命中 override，本函数只影响无 override 时的默认档位选择。
func resolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) policy.RiskLevel {
	if svcCtx.RegistryStore != nil {
		if op, err := svcCtx.RegistryStore.GetOpenAPI(functionID); err == nil && op != nil {
			if riskVal, ok := op.Extensions["x-risk"].(string); ok && strings.TrimSpace(riskVal) != "" {
				return PolicyRiskFromContract(riskVal)
			}
		}
	}
	if svcCtx.DB != nil {
		gameID, env = strings.TrimSpace(gameID), strings.TrimSpace(env)
		if gameID != "" && env != "" {
			contract, err := model.NewFunctionContractModel(svcCtx.DB).
				FindByScopeAndFunctionID(ctx, gameID, env, strings.TrimSpace(functionID))
			if err == nil && contract != nil {
				switch contract.Risk {
				case dbenum.RiskWarning, dbenum.RiskHigh, dbenum.RiskDanger:
					return PolicyRiskFromContract(contract.Risk.String())
				}
			}
		}
	}
	return policy.RiskMedium
}

// ResolvePolicyRisk exposes resolvePolicyRisk to the approve path
// (internal/api/approval step-up gate, OPEN-ISSUES #75): 高危审批门槛必须与
// 调用路径共用同一风险判定口径，禁止各包私抄解析逻辑。
func ResolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) policy.RiskLevel {
	return resolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)
}

// EnforceFunctionPolicy checks if the user's roles are allowed to invoke the function
// based on the effective policy for that function.
// Returns the effective policy for auditing purposes.
//
// ARCH-025 fail-closed: policy storage/config failure is NOT "no policy". The
// previous `return nil, nil` on GetPolicy error silently bypassed approval/audit
// for high-risk functions (function_policies table missing, SQL failure, db
// down). We now propagate the error so the caller refuses the invocation. The
// "no coverage policy" case is distinct and handled inside GetPolicy: no rows
// for the function falls back to the risk-level default policy (never nil),
// so a nil policy here only ever accompanies a resolution error.
func EnforceFunctionPolicy(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string, userRoles []string) (*policy.Policy, error) {
	if svcCtx == nil || svcCtx.PolicyManager == nil {
		return nil, fmt.Errorf("policy manager unavailable: cannot resolve effective policy for %s (config/init failure)", functionID)
	}
	riskLevel := resolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)

	// Get effective policy (for approval/audit settings)
	functionPolicy, err := svcCtx.PolicyManager.GetPolicy(ctx, functionID, riskLevel)
	if err != nil {
		return nil, fmt.Errorf("resolve effective policy for %s failed: %w", functionID, err)
	}

	// If no role restriction, allow
	if len(functionPolicy.AllowedRoles) == 0 {
		return functionPolicy, nil
	}

	// Admin role bypasses all policy checks
	if utils.HasAdminRole(userRoles) {
		return functionPolicy, nil
	}

	// Use Casbin for unified permission check if AdminModel available
	if svcCtx.AdminModel != nil {
		// Check function:invoke permission via Casbin
		_, _, err := utils.RequireAnyPermission(ctx, svcCtx, "无权调用该函数", "function:invoke")
		if err != nil {
			return nil, err
		}
		// Casbin allows, but also verify against AllowedRoles for function-specific restriction
		// This ensures function-level policies are respected
		admin, _, loadErr := utils.LoadCurrentAdmin(ctx, svcCtx)
		if loadErr == nil {
			adminRoles, roleErr := svcCtx.GetAdminRolesCached(ctx, admin.ID)
			if roleErr == nil {
				roleNames := utils.RoleNamesFromModels(adminRoles)
				if !MatchAnyRole(roleNames, functionPolicy.AllowedRoles) {
					return nil, utils.NewPermissionDenied(
						"无权调用该函数（需要角色: "+strings.Join(functionPolicy.AllowedRoles, ", ")+"）",
						map[string]any{
							"functionId":   functionID,
							"allowedRoles": functionPolicy.AllowedRoles,
						},
					)
				}
			}
		}
		return functionPolicy, nil
	}

	// Fallback: simple role matching (for tests or when AdminModel not available)
	if MatchAnyRole(userRoles, functionPolicy.AllowedRoles) {
		return functionPolicy, nil
	}

	return nil, utils.NewPermissionDenied("无权调用该函数", map[string]any{
		"functionId":   functionID,
		"allowedRoles": functionPolicy.AllowedRoles,
	})
}

// MatchAnyRole checks if userRoles contains any of the allowedRoles (case-insensitive)
func MatchAnyRole(userRoles []string, allowedRoles []string) bool {
	for _, allowed := range allowedRoles {
		for _, role := range userRoles {
			if strings.EqualFold(strings.TrimSpace(role), strings.TrimSpace(allowed)) {
				return true
			}
		}
	}
	return false
}
