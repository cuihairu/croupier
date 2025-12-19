// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AgentMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentMetricsLogic {
	return &AgentMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentMetricsLogic) AgentMetrics(req *types.AgentMetricsRequest) (*types.AgentMetricsResponse, error) {
	if req == nil || strings.TrimSpace(req.AgentId) == "" {
		return nil, errors.New("agent_id 不能为空")
	}

	agentID := strings.TrimSpace(req.AgentId)
	configuredID := strings.TrimSpace(l.svcCtx.Config.Agent.ID)
	if configuredID != "" && agentID != configuredID {
		return nil, errors.New("agent_id mismatch")
	}

	functionCount := 0
	instanceCount := 0
	if l.svcCtx.Core != nil && l.svcCtx.Core.Store() != nil {
		snap := l.svcCtx.Core.Store().List()
		functionCount = len(snap)
		for _, arr := range snap {
			instanceCount += len(arr)
		}
	}

	metrics := map[string]interface{}{
		"agentId":        agentID,
		"uptime_sec":     int64(l.svcCtx.Uptime().Seconds()),
		"functions":      int64(functionCount),
		"instances":      int64(instanceCount),
		"grpc_addr":      l.svcCtx.LocalGRPCAddr,
		"upstream_addr":  strings.TrimSpace(l.svcCtx.Config.Server.Addr),
		"heartbeat_sec":  l.svcCtx.Config.Upstream.HeartbeatInterval,
		"registered_at":  "",
		"last_heartbeat": "",
	}
	if l.svcCtx.Core == nil {
		metrics["status"] = "stopped"
	} else {
		metrics["status"] = "running"
	}

	return &types.AgentMetricsResponse{
		Metrics: metrics,
	}, nil
}
