// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

type OpsAgentProcessRestartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重启 Agent 进程
func NewOpsAgentProcessRestartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessRestartLogic {
	return &OpsAgentProcessRestartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessRestartLogic) OpsAgentProcessRestart(req *types.OpsProcessActionRequest) (*types.OpsProcessActionResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &types.OpsProcessActionResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	restartReq := &opsv1.RestartProcessRequest{
		ProcessName: req.Name,
		Force:       req.Force,
	}

	resp, err := client.RestartProcess(l.ctx, restartReq)
	if err != nil {
		return &types.OpsProcessActionResponse{
			Code:    500,
			Message: "Failed to restart process: " + err.Error(),
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
		Data:    resp.NewPid,
	}, nil
}
