package function

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	functioninvoke "github.com/cuihairu/croupier/internal/platform/functioninvoke"
	"github.com/cuihairu/croupier/internal/svc"
)

// FunctionInvokeLogic 是历史 logic 侧调用入口的薄壳。
//
// 2026-10-22 收口：函数调用执行链唯一实现在 internal/platform/functioninvoke
// （与生产在用的 internal/api/function HTTP 入口同一实现，scope、禁用、分配、
// 策略审批、审计与执行留痕同源）。本类型不再持有旁路执行实现，仅做 DTO 适配
// 与转发；入口行为差异：
//   - scope：程序化入口由调用方显式给出 (gameId, env)，转发前经
//     RequireGameEnvScope 授权（等价 HTTP 侧 GameDBMiddleware 的解析+授权），
//     然后以 ctx scope 进入统一执行链，请求体值不再单独参与路由；
//   - 禁用/分配/审批/审计/留痕：完全沿用统一执行链，不再有旧实现的无审批
//     直执行旁路；mode 语义随生产入口（仅 async 走任务，task/start_task 按
//     同步派发）。
type FunctionInvokeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用函数（薄壳转发）
func NewFunctionInvokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInvokeLogic {
	return &FunctionInvokeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInvokeLogic) FunctionInvoke(req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	if req == nil {
		req = &FunctionInvokeRequest{}
	}
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)

	// 入口 scope 授权（HTTP 侧由 GameDBMiddleware 承担的同一道闸）。
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	if err := utils.RequireGameEnvScope(l.ctx, l.svcCtx, admin.ID, roleNames, gameID, env); err != nil {
		return nil, err
	}

	// 程序化入口以显式 scope 进入统一执行链（等价中间件解析后的可信 scope）。
	ctx := svc.WithGameScope(l.ctx, svc.GameScope{GameID: gameID, Env: env})
	resp, err := functioninvoke.Invoke(ctx, l.svcCtx, &functioninvoke.Request{
		ID:              functionID,
		Params:          req.Params,
		Payload:         req.Payload,
		GameID:          gameID,
		Env:             env,
		Mode:            req.Mode,
		Route:           req.Route,
		TargetServiceID: req.TargetServiceID,
		HashKey:         req.HashKey,
		Metadata:        req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	return functionInvokeResponseFromUnified(resp), nil
}

func functionInvokeResponseFromUnified(resp *functioninvoke.Response) *FunctionInvokeResponse {
	if resp == nil {
		return nil
	}
	out := &FunctionInvokeResponse{
		Result:           resp.Result,
		TaskId:           resp.TaskId,
		ApprovalID:       resp.ApprovalID,
		ApprovalRequired: resp.ApprovalRequired,
		ApprovalWorkflow: resp.ApprovalWorkflow,
	}
	if resp.Broadcast != nil {
		items := make([]BroadcastAgentItem, 0, len(resp.Broadcast.Results))
		for _, item := range resp.Broadcast.Results {
			items = append(items, BroadcastAgentItem{AgentID: item.AgentID, Result: item.Result, Error: item.Error})
		}
		out.Broadcast = &BroadcastResult{
			Total:   resp.Broadcast.Total,
			Success: resp.Broadcast.Success,
			Failure: resp.Broadcast.Failure,
			Results: items,
		}
		// 与旧行为一致：首个成功响应镜像进 Result，兼容只读 Result 的调用方。
		out.Result = resp.Result
	}
	return out
}
