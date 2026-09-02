package function

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionInvokeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用函数
func NewFunctionInvokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInvokeLogic {
	return &FunctionInvokeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInvokeLogic) FunctionInvoke(req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}

	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)

	payload, err := invokePayloadBytes(req)
	if err != nil {
		return nil, err
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	route := strings.ToLower(strings.TrimSpace(req.Route))
	if route == "" {
		route = "lb"
	}

	metadata := map[string]string{}
	if gameID != "" {
		metadata["gameId"] = gameID
	}
	if env != "" {
		metadata["env"] = env
	}
	switch route {
	case "lb":
		// no-op
	case "targeted":
		sid := strings.TrimSpace(req.TargetServiceID)
		if sid == "" {
			return nil, errorx.NewBadRequest("target_service_id is required for route=targeted")
		}
		metadata["targetServiceId"] = sid
	case "hash":
		key := strings.TrimSpace(req.HashKey)
		if key == "" {
			return nil, errorx.NewBadRequest("hash_key is required for route=hash")
		}
		metadata["hashKey"] = key
	case "broadcast":
		// no metadata required; fan-out happens via Dispatcher.InvokeBroadcast.
	default:
		return nil, errorx.NewBadRequest("invalid route " + route)
	}

	if route == "broadcast" && (mode == "task" || mode == "start_task" || mode == "async") {
		return nil, errorx.NewBadRequest("route=broadcast is only supported for synchronous invoke")
	}

	if mode == "task" || mode == "start_task" || mode == "async" {
		if err := utils.RequireGameEnvScope(l.ctx, l.svcCtx, admin.ID, roleNames, gameID, env); err != nil {
			return nil, err
		}
		if err := l.enforceInvokePermission(roleNames, permIDs, functionID, gameID, env); err != nil {
			return nil, err
		}
		taskResp, err := l.svcCtx.Dispatcher.StartTaskRequest(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
		if err != nil {
			return nil, err
		}
		taskID := taskResp.GetTaskId()
		return &FunctionInvokeResponse{TaskId: taskID, TaskID: taskID}, nil
	}

	// Default: synchronous invoke.
	if err := utils.RequireGameEnvScope(l.ctx, l.svcCtx, admin.ID, roleNames, gameID, env); err != nil {
		return nil, err
	}
	if err := l.enforceInvokePermission(roleNames, permIDs, functionID, gameID, env); err != nil {
		return nil, err
	}

	if route == "broadcast" {
		broadcast, err := l.svcCtx.Dispatcher.InvokeBroadcast(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
		if err != nil {
			return nil, err
		}
		return buildBroadcastResponse(broadcast), nil
	}

	resp, err := l.svcCtx.Dispatcher.InvokeRequest(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
	if err != nil {
		return nil, err
	}
	out := &FunctionInvokeResponse{}
	if resp != nil && len(resp.GetPayload()) > 0 {
		out.Result = rawJSONFromBytes(resp.GetPayload())
	}
	return out, nil
}

func (l *FunctionInvokeLogic) enforceInvokePermission(roleNames []string, permIDs []string, functionID string, gameID string, env string) error {
	return utils.CheckInvokePermission(l.ctx, l.svcCtx, roleNames, permIDs, functionID, gameID, env)
}

// buildBroadcastResponse aggregates per-agent outcomes and also populates
// Result with the first successful response so legacy callers that don't
// know about Broadcast keep working.
func buildBroadcastResponse(b *dispatch.BroadcastInvocation) *FunctionInvokeResponse {
	if b == nil {
		return &FunctionInvokeResponse{Broadcast: &BroadcastResult{}}
	}

	out := &FunctionInvokeResponse{
		Broadcast: &BroadcastResult{
			Total:   b.Total,
			Success: len(b.Successes),
			Failure: len(b.Failures),
			Results: make([]BroadcastAgentItem, 0, b.Total),
		},
	}

	for _, s := range b.Successes {
		item := BroadcastAgentItem{AgentID: s.AgentID}
		if s.Response != nil && len(s.Response.GetPayload()) > 0 {
			item.Result = rawJSONFromBytes(s.Response.GetPayload())
			if out.Result == nil {
				out.Result = item.Result
			}
		}
		out.Broadcast.Results = append(out.Broadcast.Results, item)
	}

	for _, f := range b.Failures {
		out.Broadcast.Results = append(out.Broadcast.Results, BroadcastAgentItem{
			AgentID: f.AgentID,
			Error:   f.Err.Error(),
		})
	}

	return out
}

func invokePayloadBytes(req *FunctionInvokeRequest) ([]byte, error) {
	if req == nil {
		return []byte("null"), nil
	}
	switch {
	case len(req.Payload) > 0:
		if !jsonValid(req.Payload) {
			return nil, errorx.NewBadRequest("payload must be valid JSON")
		}
		return append([]byte(nil), req.Payload...), nil
	case len(req.Params) > 0:
		if !jsonValid(req.Params) {
			return nil, errorx.NewBadRequest("params must be valid JSON")
		}
		return append([]byte(nil), req.Params...), nil
	default:
		return []byte("{}"), nil
	}
}

func jsonValid(raw []byte) bool {
	return len(raw) > 0 && json.Valid(raw)
}
