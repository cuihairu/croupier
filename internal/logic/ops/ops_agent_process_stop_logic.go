
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
	

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

type OpsAgentProcessStopLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 停止 Agent 进程
func NewOpsAgentProcessStopLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStopLogic {
	return &OpsAgentProcessStopLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStopLogic) OpsAgentProcessStop(req *OpsProcessActionRequest) (*OpsProcessActionResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &OpsProcessActionResponse{
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
		return &OpsProcessActionResponse{
			Code:    500,
			Message: "Failed to stop process: " + err.Error(),
		}, nil
	}

	if !resp.Success {
		return &OpsProcessActionResponse{
			Code:    500,
			Message: resp.Message,
		}, nil
	}

	return &OpsProcessActionResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
