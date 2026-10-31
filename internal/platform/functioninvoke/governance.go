package functioninvoke

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	logicassignment "github.com/cuihairu/croupier/internal/logic/assignment"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

// Subject 描述一次调用的执行主体。
//
//   - 人类操作员：Admin 非 nil，Roles 为其角色（普通 invoke、/tasks Start、
//     schedule Create/TriggerNow）。
//   - 自动化主体：Admin 为 nil，AutomatedID 非空（cron 调度触发，如
//     "schedule:42"）。无独立权限 ID；角色由调用方经 creator 回载提供，
//     供策略 AllowedRoles 判定。
type Subject struct {
	Admin       *model.Admin
	Roles       []model.Role
	PermIDs     []string // 预计算权限 ID；nil 时由 Gate 从 Roles 计算
	AutomatedID string
}

// IsAutomated 报告主体是否为自动化触发者（cron）。
func (s Subject) IsAutomated() bool { return s.Admin == nil && s.AutomatedID != "" }

// RoleNames 返回主体角色名（自动化主体用回载角色）。
func (s Subject) RoleNames() []string { return utils.RoleNamesFromModels(s.Roles) }

// GateInput 是一次将要派发的调用的治理输入。
type GateInput struct {
	FunctionID string
	GameID     string
	Env        string
	Payload    []byte
	Metadata   map[string]string
}

// GateResult 返回治理结论。
type GateResult struct {
	FunctionPolicy *policy.Policy
	Admin          *model.Admin // 人类主体；自动化主体为 nil
	RoleNames      []string
	// RequireApproval：有效政策要求审批，且调用未携带批准快照/页面治理豁免。
	// 人类调用方据此创建审批请求；自动化调用方（cron）据此跳过派发。
	RequireApproval  bool
	ApprovalWorkflow string
}

// Governance 是跨入口共享的执行治理闸门（ARCH-024）：普通 invoke、/tasks Start、
// schedule Create/TriggerNow/自动触发 都经同一闸门，保证同一高危函数在各入口
// 一致执行 禁用→分配→函数权限→政策审批→输入契约，不把模块级 JWT/scope 误作
// 函数政策。函数级政策与 scope 授权是两个正交维度（scope 由入口中间件/显式
// RequireGameEnvScope 承担，本闸门只管函数级治理）。
type Governance struct {
	svcCtx *svc.ServiceContext
}

// NewGovernance 构造治理闸门。
func NewGovernance(svcCtx *svc.ServiceContext) *Governance {
	return &Governance{svcCtx: svcCtx}
}

// Gate 执行统一治理链：禁用拦截 → 分配闸门 → 函数权限（人类主体）→ 政策
// （fail-closed）→ 输入契约。
//
// 自动化主体（cron）不查独立函数权限 ID（无人类操作员），其权限由调用方回载的
// 角色经 EnforceFunctionPolicy 的 AllowedRoles 判定；政策始终回读（live）——
// 批准快照（approvalBypass=approved）仅满足审批要求，不豁免政策本身，策略变更
// 仍被捕获（满足「撤权后不继续裸派发」）。人类主体沿用 approved continuation /
// 页面快照治理豁免（上游已把关，不二次审批）。
func (g *Governance) Gate(ctx context.Context, subject Subject, in GateInput) (*GateResult, error) {
	functionID := strings.TrimSpace(in.FunctionID)
	if functionID == "" {
		return nil, errorx.NewBadRequest("function id is required")
	}
	gameID := strings.TrimSpace(in.GameID)
	env := strings.TrimSpace(in.Env)

	// 1. 禁用拦截（E2）：dashboard 禁用写 functions.status，未分配/禁用函数
	//    在各入口一律拒绝，不允许任何入口绕过。
	if err := utils.EnsureFunctionEnabled(ctx, g.svcCtx, functionID); err != nil {
		return nil, err
	}
	// 2. 分配闸门（BUG-032）：scope 有分配记录时是执行白名单，未分配 403。
	if err := logicassignment.EnsureFunctionAssigned(ctx, g.svcCtx, functionID, gameID, env); err != nil {
		return nil, err
	}

	// 3. 函数权限（人类主体）：Casbin function:invoke + 角色匹配。
	roleNames := subject.RoleNames()
	if !subject.IsAutomated() {
		if subject.Admin == nil {
			admin, roles, err := utils.LoadCurrentAdmin(ctx, g.svcCtx)
			if err != nil {
				return nil, err
			}
			subject.Admin, subject.Roles = admin, roles
			roleNames = subject.RoleNames()
		}
		permIDs := subject.PermIDs
		if permIDs == nil {
			var err error
			permIDs, err = utils.PermissionIDsFromRoles(ctx, g.svcCtx, subject.Roles)
			if err != nil {
				return nil, err
			}
		}
		if err := utils.CheckInvokePermission(ctx, g.svcCtx, roleNames, permIDs, functionID, gameID, env); err != nil {
			return nil, err
		}
	}

	// 4. 政策（fail-closed，ARCH-025）：政策存储/配置故障一律拒绝，不与
	//    「无覆盖策略」混淆（无行回落风险级默认策略，永不 nil）。
	var functionPolicy *policy.Policy
	var err error
	if subject.IsAutomated() {
		// 自动化主体：始终回读政策（live）。角色由调用方回载（creator）。
		functionPolicy, err = EnforceFunctionPolicy(ctx, g.svcCtx, functionID, gameID, env, roleNames)
		if err != nil {
			return nil, err
		}
	} else if !IsApprovedContinuation(in.Metadata) && !IsPageSnapshotGoverned(in.Metadata) {
		functionPolicy, err = EnforceFunctionPolicy(ctx, g.svcCtx, functionID, gameID, env, roleNames)
		if err != nil {
			return nil, err
		}
	}

	// 5. 输入契约：缺 required 参数在派发前拦截（400 指明缺失字段）。
	if err := ValidateInvokeInput(ctx, g.svcCtx, &Request{ID: functionID}, in.Payload); err != nil {
		return nil, err
	}

	result := &GateResult{
		FunctionPolicy: functionPolicy,
		Admin:          subject.Admin,
		RoleNames:      roleNames,
	}
	// 批准快照/页面治理豁免：approvalBypass=approved 或 pageSnapshotGovernance
	// 已由上游把关，不再要求二次审批。
	hasSnapshot := IsApprovedContinuation(in.Metadata)
	if functionPolicy != nil && functionPolicy.RequireApproval && !hasSnapshot {
		result.RequireApproval = true
		result.ApprovalWorkflow = functionPolicy.ApprovalWorkflow
	}
	return result, nil
}

// CheckGovernance 是 cron 调度触发用的闸门入口（自动化主体）：以调度 creator
// 回载角色执行统一治理。返回 skip=true 表示不应派发（政策要求审批且无批准快照，
// 或 creator 角色已被撤权）；err 非 nil 表示治理故障（fail-closed，同样不派发）。
//
// 与 human 入口的差异：主体为自动化（无独立权限 ID），批准快照仅满足审批要求，
// 政策始终 live 回读。automatedID 为显式主体标识（如 "schedule:42"）。
func (g *Governance) CheckGovernance(ctx context.Context, functionID, gameID, env string, payload []byte, metadata map[string]string, actor, automatedID string) (bool, error) {
	subject := Subject{AutomatedID: automatedID}
	if strings.TrimSpace(actor) != "" && actor != "system" {
		// 以调度 creator 回载角色：creator 被删除/撤权 → 角色为空 → 策略
		// AllowedRoles 拒绝 → 跳过派发（fail-closed）。注入 username 供
		// LoadCurrentAdmin 解析。
		actorCtx := context.WithValue(ctx, "username", actor)
		if admin, roles, err := utils.LoadCurrentAdmin(actorCtx, g.svcCtx); err == nil && admin != nil {
			subject.Roles = roles
		}
	}
	res, err := g.Gate(ctx, subject, GateInput{
		FunctionID: functionID,
		GameID:     gameID,
		Env:        env,
		Payload:    payload,
		Metadata:   metadata,
	})
	if err != nil {
		return true, err
	}
	if res.RequireApproval {
		return true, nil // 政策要求审批且无批准快照 → 跳过派发
	}
	return false, nil
}
