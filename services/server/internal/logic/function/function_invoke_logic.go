// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionInvokeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用函数
func NewFunctionInvokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInvokeLogic {
	return &FunctionInvokeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInvokeLogic) FunctionInvoke(req *types.FunctionInvokeRequest) (*types.FunctionInvokeResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

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
	switch route {
	case "lb":
		// no-op
	case "targeted":
		sid := strings.TrimSpace(req.TargetServiceId)
		if sid == "" {
			return nil, fmt.Errorf("target_service_id is required for route=targeted")
		}
		metadata["target_service_id"] = sid
	case "hash":
		key := strings.TrimSpace(req.HashKey)
		if key == "" {
			return nil, fmt.Errorf("hash_key is required for route=hash")
		}
		metadata["hash_key"] = key
	case "broadcast":
		return nil, fmt.Errorf("route=broadcast not implemented")
	default:
		return nil, fmt.Errorf("invalid route %q", route)
	}

	if mode == "job" || mode == "start_job" || mode == "async" {
		jobResp, err := l.svcCtx.Dispatcher.StartJobRequest(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
		if err != nil {
			return nil, err
		}
		jobID := jobResp.GetJobId()
		return &types.FunctionInvokeResponse{JobId: jobID, JobID: jobID}, nil
	}

	// Default: synchronous invoke.
	resp, err := l.svcCtx.Dispatcher.InvokeRequest(l.ctx, utils.BuildInvokeRequest(functionID, payload, metadata))
	if err != nil {
		return nil, err
	}
	out := &types.FunctionInvokeResponse{}
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
