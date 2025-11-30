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

	state := l.svcCtx.AgentState
	state.Mu.RLock()
	info := state.Agents[req.AgentId]
	totalJobs := len(state.Jobs)
	running := 0
	for _, job := range state.Jobs {
		if job.Status == "running" {
			running++
		}
	}
	state.Mu.RUnlock()

	metrics := map[string]interface{}{
		"agentId":        req.AgentId,
		"uptime_sec":     int64(state.Uptime().Seconds()),
		"jobs_total":     totalJobs,
		"jobs_running":   running,
		"functions":      int64(0),
		"last_heartbeat": "",
		"registered_at":  "",
	}

	if info != nil {
		metrics["status"] = info.Status
		metrics["functions"] = info.Functions
		metrics["metadata"] = info.Metadata
		metrics["last_heartbeat"] = formatTime(info.LastHeartbeat)
		metrics["registered_at"] = formatTime(info.RegisteredAt)
	} else {
		metrics["status"] = "unknown"
	}

	return &types.AgentMetricsResponse{
		Metrics: metrics,
	}, nil
}
