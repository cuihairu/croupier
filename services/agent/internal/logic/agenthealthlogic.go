package logic

import (
	"context"
	"runtime"
	"time"

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

func (l *AgentHealthLogic) AgentHealth(req *types.AgentHealthRequest) (resp *types.AgentHealthResponse, err error) {
	logx.Infof("Health check requested for agent: %s", req.AgentId)

	// Check if agent exists and get registration info
	agentInfo, exists := l.svcCtx.AgentStore.GetAgentInfo(req.AgentId)
	if !exists {
		return &types.AgentHealthResponse{
			Status:    "unknown",
			Uptime:    0,
			Jobs:      0,
			Functions: 0,
			Memory:    0,
			Cpu:       0,
		}, nil
	}

	// Get agent registration time to calculate uptime
	var uptime int64
	if info, ok := agentInfo.(map[string]interface{}); ok {
		if registeredAt, ok := info["registered_at"].(int64); ok {
			uptime = time.Now().Unix() - registeredAt
		}
	}

	// Get current job count
	jobs := l.svcCtx.JobManager

	// Get memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryUsage := m.Alloc / 1024 / 1024 // Convert to MB

	// Get function count
	functions := l.svcCtx.AgentStore.ListFunctions()

	status := "healthy"
	// Check last heartbeat
	if info, ok := agentInfo.(map[string]interface{}); ok {
		if lastHeartbeat, ok := info["last_heartbeat"].(int64); ok {
			if time.Now().Unix()-lastHeartbeat > int64(l.svcCtx.Config.Upstream.HeartbeatInterval*2) {
				status = "unhealthy"
			}
		}
	}

	return &types.AgentHealthResponse{
		Status:    status,
		Uptime:    uptime,
		Jobs:      int64(len(jobs.ListJobs())),
		Functions: int64(len(functions)),
		Memory:    int64(memoryUsage),
		Cpu:       0.0, // TODO: Implement CPU usage calculation
	}, nil
}