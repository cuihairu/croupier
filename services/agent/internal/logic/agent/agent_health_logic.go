// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentHealthLogic {
	return &AgentHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentHealthLogic) AgentHealth(req *types.AgentHealthRequest) (*types.AgentHealthResponse, error) {
	if req == nil || strings.TrimSpace(req.AgentId) == "" {
		return nil, errors.New("agent_id 不能为空")
	}

	state := l.svcCtx.AgentState
	state.Mu.RLock()
	info := state.Agents[req.AgentId]
	activeJobs := 0
	for _, job := range state.Jobs {
		if job.Status == "running" {
			activeJobs++
		}
	}
	state.Mu.RUnlock()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := int64(state.Uptime().Seconds())
	functions := int64(0)
	status := "unknown"
	if info != nil {
		functions = info.Functions
		status = info.Status
	}
	if uptime < 0 {
		uptime = 0
	}

	return &types.AgentHealthResponse{
		Status:    status,
		Uptime:    uptime,
		Jobs:      int64(activeJobs),
		Functions: functions,
		Memory:    int64(mem.Alloc),
		Cpu:       0,
	}, nil
}
