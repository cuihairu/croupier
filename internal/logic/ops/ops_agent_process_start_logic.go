
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
	

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

type OpsAgentProcessStartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启动 Agent 进程
func NewOpsAgentProcessStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessStartLogic {
	return &OpsAgentProcessStartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessStartLogic) OpsAgentProcessStart(req *OpsProcessStartRequest) (*OpsProcessStartResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &OpsProcessStartResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	startReq := &opsv1.StartProcessRequest{
		ProcessName: req.Name,
	}

	resp, err := client.StartProcess(l.ctx, startReq)
	if err != nil {
		return &OpsProcessStartResponse{
			Code:    500,
			Message: "Failed to start process: " + err.Error(),
		}, nil
	}

	if !resp.Success {
		return &OpsProcessStartResponse{
			Code:    500,
			Message: resp.Message,
		}, nil
	}

	return &OpsProcessStartResponse{
		Code:    0,
		Message: "OK",
		Data:    resp.Pid,
	}, nil
}
