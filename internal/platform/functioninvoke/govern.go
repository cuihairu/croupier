package functioninvoke

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/assignment"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

// SubjectKind 区分执行准入的主体形态（ARCH-024）。
type SubjectKind string

const (
	// SubjectHuman 是交互式入口（HTTP /tasks Start / 手动调度 / 页面执行）：
	// 主体从 ctx 中间件解析（username 由鉴权中间件注入）。
	SubjectHuman SubjectKind = "human"
	// SubjectAutomation 是自动化入口（cron 调度自动触发）：主体由调度定义
	// 携带的创建者（Actor）决定，每次触发以该创建者身份重新授权——撤权后
	// 不继续裸派发。
	SubjectAutomation SubjectKind = "automation"
)

// Subject 是执行准入的显式主体（R01：调度中的 actor 字符串不等于每次
// 触发重新授权，必须显式定义自动化主体并逐次重载）。
type Subject struct {
	Kind          SubjectKind
	ActorUsername string // SubjectAutomation：调度创建者用户名（空 = 拒绝）
}

// GovernResult 是 Govern 的准入判定结果：通过时携带后续派发/审计所需的
// 主体、角色与生效政策；ApprovalRequired=true 时调用方应转入审批流而
// 不派发。
type GovernResult struct {
	Admin            *model.Admin
	Roles            []model.Role
	RoleNames        []string
	FunctionPolicy   *policy.Policy
	ApprovalRequired bool
	ApprovalID       string
	ApprovalWorkflow string
}

// governPhase 划分准入链的阶段（ARCH-024：各入口复用同一套门，仅尾部
// 授权/审批深度不同）。
type governPhase int

const (
	// phaseFull 完整链：主体 + 政策审批 + 审批创建（Invoke / Start /
	// 手动调度 / 自动触发等执行入口）。
	phaseFull governPhase = iota
	// phaseInvocability 仅校验「函数可调度」：禁用/分配/输入契约，不加载
	// 主体、不跑政策、不建审批（调度 Create 定义期入口）。授权与政策审批
	// 由每次触发重新授权承担，定义期不做执行授权。
	phaseInvocability
)

// Govern 是任务/调度/页面/HTTP 共用的执行准入链（ARCH-024）：禁用拦截 →
// 分配闸门 → 主体加载 → 函数权限/政策审批 → 输入契约 → 审批判定。
// 与 functioninvoke.Invoke 内联门序一致（行为保持），但不派发、不审计、
// 不写执行留痕——那些是调用方（Invoke / task.Start / 调度 Manager）的职责。
//
// 门序（与 Invoke 既有顺序逐条对齐）：
//  1. EnsureFunctionEnabled     —— dashboard 禁用函数不执行（E2）；
//  2. EnsureFunctionAssigned    —— scope 分配白名单（BUG-032）；
//  3. LoadCurrentAdmin          —— 主体加载（admin+roles）；
//  4. PermissionIDsFromRoles    —— 权限装载错误面保持；
//  5. EnforceFunctionPolicy     —— 政策审批（approvedContinuation /
//     pageSnapshotGoverned 跳过二次审批，同 Invoke）；
//  6. ValidateInvokeInput       —— 输入契约（缺 required 参数 400）；
//  7. CreateFunctionApproval    —— RequireApproval 时创建审批、不派发。
//
// 环境 scope（RequireGameEnvScope）有意不在此链内：HTTP 入口由鉴权中间件
// 的 CurrentScope 前置授权，/tasks Start 由服务自身 RequireGameEnvScope
// 承担，两处均已按入口独立处理，Govern 不重复加锁以避免双读与测试面
// 漂移。仅 SubjectAutomation 补一次创建者 scope 复核（每次触发重新授权，
// 撤权后不继续裸派发）。
//
// subject 语义：
//   - SubjectHuman：admin/roles 从 ctx（鉴权中间件）解析，原样透传；
//   - SubjectAutomation：以 ActorUsername 派生治理 ctx（stale-heal 同款
//     context.WithValue(ctx, "username", actor)），每次调用重载该主体当前
//     角色与权限——撤权/改角色即时生效，而非沿用创建时的 actor 字符串。
func Govern(ctx context.Context, svcCtx *svc.ServiceContext, req *Request, sub Subject) (*GovernResult, error) {
	return govern(ctx, svcCtx, req, sub, phaseFull)
}

// GovernInvocability 是调度 Create 的定义期校验（仅禁用/分配/输入契约）：
// 验证「该函数当前可调度」，不加载主体、不跑政策审批、不建审批记录。
// 授权与政策审批由每次触发重新授权承担——定义期不做执行授权，也不会因
// 审批创建污染审批队列（审批是执行期动作）。
func GovernInvocability(ctx context.Context, svcCtx *svc.ServiceContext, req *Request) (*GovernResult, error) {
	return govern(ctx, svcCtx, req, Subject{}, phaseInvocability)
}

func govern(ctx context.Context, svcCtx *svc.ServiceContext, req *Request, sub Subject, phase governPhase) (*GovernResult, error) {
	// 1. 治理 ctx：自动化主体以创建者身份注入（page/stale_heal.go:106 先例）。
	gctx := ctx
	if phase == phaseFull && sub.Kind == SubjectAutomation {
		actor := strings.TrimSpace(sub.ActorUsername)
		if actor == "" {
			return nil, errorx.NewBadRequest("automation subject requires an actor")
		}
		gctx = context.WithValue(ctx, "username", actor)
	}

	functionID := strings.TrimSpace(req.ID)
	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)

	// 2. 禁用拦截（E2）：被拒的调用不是一次执行，不产生失败执行记录。
	if err := utils.EnsureFunctionEnabled(gctx, svcCtx, functionID); err != nil {
		return nil, err
	}
	// 3. 分配闸门（BUG-032）。
	if err := assignment.EnsureFunctionAssigned(gctx, svcCtx, functionID, gameID, env); err != nil {
		return nil, err
	}

	// 4. 输入契约（禁用/分配之后、派发之前校验；缺 required 参数 400）。
	payload := InvokePayload(req)
	if err := ValidateInvokeInput(gctx, svcCtx, req, payload); err != nil {
		return nil, err
	}

	if phase == phaseInvocability {
		// 定义期校验到此为止：不加载主体、不跑政策、不建审批。
		return &GovernResult{}, nil
	}

	// 5. 主体加载。
	admin, roles, err := utils.LoadCurrentAdmin(gctx, svcCtx)
	if err != nil {
		return nil, err
	}
	if _, err := utils.PermissionIDsFromRoles(gctx, svcCtx, roles); err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)

	// 6. 自动化主体补 scope 复核：每次触发以创建者当前权限复核其对
	// game/env 的访问，撤权后不再派发。Human 由入口中间件/服务承担。
	if sub.Kind == SubjectAutomation {
		if err := utils.RequireGameEnvScope(gctx, svcCtx, admin.ID, roleNames, gameID, env); err != nil {
			return nil, err
		}
	}

	// 7. 函数权限/政策审批（跳过续跑与页面快照治理的二次审批，同 Invoke）。
	var functionPolicy *policy.Policy
	if svcCtx.PolicyManager != nil && !IsApprovedContinuation(req.Metadata) && !IsPageSnapshotGoverned(req.Metadata) {
		functionPolicy, err = EnforceFunctionPolicy(gctx, svcCtx, functionID, gameID, env, roleNames)
		if err != nil {
			return nil, err
		}
	}

	// 8. 审批判定：RequireApproval 时创建审批请求、返回待审（不派发）。
	if functionPolicy != nil && functionPolicy.RequireApproval && svcCtx.ApprovalsStore != nil {
		approvalID, err := CreateFunctionApproval(gctx, svcCtx, req, payload, admin, functionPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to create approval request: %w", err)
		}
		if svcCtx.AuditService != nil && functionPolicy.RequireAudit {
			AuditApprovalCreated(gctx, svcCtx, functionID, admin, roleNames, approvalID, functionPolicy)
		}
		return &GovernResult{
			Admin:            admin,
			Roles:            roles,
			RoleNames:        roleNames,
			FunctionPolicy:   functionPolicy,
			ApprovalRequired: true,
			ApprovalID:       approvalID,
			ApprovalWorkflow: functionPolicy.ApprovalWorkflow,
		}, nil
	}

	return &GovernResult{
		Admin:          admin,
		Roles:          roles,
		RoleNames:      roleNames,
		FunctionPolicy: functionPolicy,
	}, nil
}
