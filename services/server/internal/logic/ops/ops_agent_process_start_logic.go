// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAgentProcessStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动 Agent 进程
func NewOpsAgentProcessStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStartLogic {
	return &OpsAgentProcessStartLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStartLogic) OpsAgentProcessStart(req *types.OpsProcessStartRequest) (*types.OpsProcessStartResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &types.OpsProcessStartResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	startReq := &opsv1.StartProcessRequest{
		ProcessName: req.Name,
	}

	resp, err := client.StartProcess(l.ctx, startReq)
	if err != nil {
		return &types.OpsProcessStartResponse{
			Code:    500,
			Message: "Failed to start process: " + err.Error(),
		}, nil
	}

	if !resp.Success {
		return &types.OpsProcessStartResponse{
			Code:    500,
			Message: resp.Message,
		}, nil
	}

	return &types.OpsProcessStartResponse{
		Code:    0,
		Message: "OK",
		Data:    resp.Pid,
	}, nil
}
