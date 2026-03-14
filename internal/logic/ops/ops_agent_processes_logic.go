package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
)

type OpsAgentProcessesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Agent 进程列表
func NewOpsAgentProcessesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAgentProcessesLogic {
	return &OpsAgentProcessesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAgentProcessesLogic) OpsAgentProcesses(req *OpsAgentProcessesRequest) (*OpsAgentProcessesResponse, error) {
	// Query agent via gRPC
	client, err := GetAgentOpsClient().GetClient(l.ctx, req.AgentID)
	if err != nil {
		return &OpsAgentProcessesResponse{
			Code:    404,
			Message: err.Error(),
		}, nil
	}

	resp, err := client.ListProcesses(l.ctx)
	if err != nil {
		return &OpsAgentProcessesResponse{
			Code:    500,
			Message: "Failed to list processes: " + err.Error(),
		}, nil
	}

	// Convert proto response to API format
	processes := make([]OpsManagedProcess, 0, len(resp.Processes))
	for _, p := range resp.Processes {
		var lastStart string
		if p.LastStart != nil {
			lastStart = p.LastStart.String()
		}

		processes = append(processes, OpsManagedProcess{
			Name:         p.Name,
			Command:      p.Command,
			WorkingDir:   p.WorkingDir,
			State:        p.State.String(),
			Pid:          p.Pid,
			RestartCount: p.RestartCount,
			LastStart:    lastStart,
		})
	}

	return &OpsAgentProcessesResponse{
		Code:    0,
		Message: "OK",
		Data:    processes,
	}, nil
}
