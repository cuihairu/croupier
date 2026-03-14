// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

type OpsAgentExecCommandLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 在 Agent 上执行命令（高风险）
func NewOpsAgentExecCommandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentExecCommandLogic {
	return &OpsAgentExecCommandLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentExecCommandLogic) OpsAgentExecCommand(req *types.OpsExecCommandRequest) (*types.OpsExecCommandResponse, error) {
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &types.OpsExecCommandResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	execReq := &opsv1.ExecuteCommandRequest{
		Command: req.Command,
		Args:    req.Args,
	}

	if req.Timeout > 0 {
		execReq.TimeoutSeconds = req.Timeout
	}

	resp, err := client.ExecuteCommand(l.ctx, execReq)
	if err != nil {
		return &types.OpsExecCommandResponse{
			Code:    500,
			Message: "Failed to execute command: " + err.Error(),
		}, nil
	}

	return &types.OpsExecCommandResponse{
		Code:    0,
		Message: "OK",
		Data: types.OpsExecCommandResult{
			Success:  resp.Success,
			ExitCode: resp.ExitCode,
			Stdout:   resp.StdOut,
			Stderr:   resp.StdErr,
		},
	}, nil
}
