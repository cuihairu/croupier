package logic

import (
	"context"
	"time"

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

func (l *AgentMetricsLogic) AgentMetrics(req *types.AgentMetricsRequest) (resp *types.AgentMetricsResponse, err error) {
	logx.Infof("Metrics requested for agent: %s", req.AgentId)

	// Check if agent exists
	agentInfo, exists := l.svcCtx.AgentStore.GetAgentInfo(req.AgentId)
	if !exists {
		return &types.AgentMetricsResponse{
			Metrics: map[string]interface{}{
				"error": "agent not found",
			},
		}, nil
	}

	// Collect metrics
	metrics := make(map[string]interface{})

	// Basic agent info
	if info, ok := agentInfo.(map[string]interface{}); ok {
		metrics["agent_id"] = info["agent_id"]
		metrics["game_id"] = info["game_id"]
		metrics["env"] = info["env"]
		metrics["status"] = info["status"]
		metrics["last_heartbeat"] = info["last_heartbeat"]
	}

	// Job metrics
	jobs := l.svcCtx.JobManager.ListJobs()
	jobMetrics := make(map[string]interface{})
	totalJobs := len(jobs)
	completedJobs := 0
	failedJobs := 0
	runningJobs := 0

	for _, job := range jobs {
		switch job.Status {
		case "completed":
			completedJobs++
		case "failed":
			failedJobs++
		case "running":
			runningJobs++
		}
	}

	jobMetrics["total"] = totalJobs
	jobMetrics["completed"] = completedJobs
	jobMetrics["failed"] = failedJobs
	jobMetrics["running"] = runningJobs
	metrics["jobs"] = jobMetrics

	// Function metrics
	functions := l.svcCtx.AgentStore.ListFunctions()
	metrics["functions"] = map[string]interface{}{
		"total": len(functions),
	}

	// System metrics
	metrics["system"] = map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(time.Now()).Seconds(), // TODO: Calculate real uptime
	}

	// Configuration metrics
	metrics["config"] = map[string]interface{}{
		"max_concurrent_jobs": l.svcCtx.Config.Job.MaxConcurrent,
		"job_timeout":        l.svcCtx.Config.Job.Timeout,
		"heartbeat_interval": l.svcCtx.Config.Upstream.HeartbeatInterval,
	}

	return &types.AgentMetricsResponse{
		Metrics: metrics,
	}, nil
}