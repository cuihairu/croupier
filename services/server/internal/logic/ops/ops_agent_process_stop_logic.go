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

type OpsAgentProcessStopLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 停止 Agent 进程
func NewOpsAgentProcessStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStopLogic {
	return &OpsAgentProcessStopLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStopLogic) OpsAgentProcessStop(req *types.OpsProcessActionRequest) (*types.OpsProcessActionResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &types.OpsProcessActionResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	stopReq := &opsv1.StopProcessRequest{
		ProcessName: req.Name,
		Force:       req.Force,
	}

	resp, err := client.StopProcess(l.ctx, stopReq)
	if err != nil {
		return &types.OpsProcessActionResponse{
			Code:    500,
			Message: "Failed to stop process: " + err.Error(),
		}, nil
	}

	if !resp.Success {
		return &types.OpsProcessActionResponse{
			Code:    500,
			Message: resp.Message,
		}, nil
	}

	return &types.OpsProcessActionResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
