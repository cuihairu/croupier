package function

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
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

	payloadObj := req.Payload
	if payloadObj == nil {
		payloadObj = req.Params
	}
	if payloadObj == nil {
		payloadObj = map[string]interface{}{}
	}

	payload, err := json.Marshal(payloadObj)
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
		metadata["game_id"] = gameID
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
		metadata["target_service_id"] = sid
	case "hash":
		key := strings.TrimSpace(req.HashKey)
		if key == "" {
			return nil, errorx.NewBadRequest("hash_key is required for route=hash")
		}
		metadata["hash_key"] = key
	case "broadcast":
		return nil, errorx.NewBadRequest("route=broadcast not implemented")
	default:
		return nil, errorx.NewBadRequest("invalid route " + route)
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
	resp, err := l.svcCtx.Dispatcher.InvokeRequest(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
	if err != nil {
		return nil, err
	}
	out := &FunctionInvokeResponse{}
	if resp != nil && len(resp.GetPayload()) > 0 {
		var v interface{}
		if err := json.Unmarshal(resp.GetPayload(), &v); err == nil {
			out.Result = v
		} else {
			out.Result = string(resp.GetPayload())
		}
	}
	return out, nil
}

func (l *FunctionInvokeLogic) enforceInvokePermission(roleNames []string, permIDs []string, functionID string, gameID string, env string) error {
	return utils.CheckInvokePermission(l.ctx, l.svcCtx, roleNames, permIDs, functionID, gameID, env)
}
