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

	agentID := strings.TrimSpace(req.AgentId)
	configuredID := strings.TrimSpace(l.svcCtx.Config.Agent.ID)
	if configuredID != "" && agentID != configuredID {
		return nil, errors.New("agent_id mismatch")
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := int64(l.svcCtx.Uptime().Seconds())
	if uptime < 0 {
		uptime = 0
	}

	functionCount := int64(0)
	if l.svcCtx.Core != nil && l.svcCtx.Core.Store() != nil {
		functionCount = int64(len(l.svcCtx.Core.Store().List()))
	}

	status := "running"
	if l.svcCtx.Core == nil {
		status = "stopped"
	}

	return &types.AgentHealthResponse{
		Status:    status,
		Uptime:    uptime,
		Jobs:      0,
		Functions: functionCount,
		Memory:    int64(mem.Alloc),
		Cpu:       0,
	}, nil
}
