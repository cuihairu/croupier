// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/emptypb"
)

type OpsAgentProcessesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 进程列表
func NewOpsAgentProcessesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessesLogic {
	return &OpsAgentProcessesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessesLogic) OpsAgentProcesses(req *types.OpsAgentProcessesRequest) (*types.OpsAgentProcessesResponse, error) {
	// Query agent via gRPC
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &types.OpsAgentProcessesResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	resp, err := client.ListProcesses(l.ctx, &emptypb.Empty{})
	if err != nil {
		return &types.OpsAgentProcessesResponse{
			Code:    500,
			Message: "Failed to list processes: " + err.Error(),
		}, nil
	}

	// Convert proto response to API format
	processes := make([]types.OpsManagedProcess, 0, len(resp.Processes))
	for _, p := range resp.Processes {
		var lastStart string
		if p.LastStart != nil {
			lastStart = p.LastStart.String()
		}

		processes = append(processes, types.OpsManagedProcess{
			Name:         p.Name,
			Command:      p.Command,
			WorkingDir:   p.WorkingDir,
			State:        p.State.String(),
			Pid:          p.Pid,
			RestartCount: p.RestartCount,
			LastStart:    lastStart,
		})
	}

	return &types.OpsAgentProcessesResponse{
		Code:    0,
		Message: "OK",
		Data:    processes,
	}, nil
}
