package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AgentHeartbeatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentHeartbeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentHeartbeatLogic {
	return &AgentHeartbeatLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentHeartbeatLogic) AgentHeartbeat(req *types.AgentHeartbeatRequest) (resp *types.AgentHeartbeatResponse, err error) {
	logx.Infof("Agent heartbeat: agentId=%s, gameId=%s, env=%s, functions=%d",
		req.AgentId, req.GameId, req.Env, req.Functions)

	// Check if agent exists
	_, exists := l.svcCtx.AgentStore.GetAgentInfo(req.AgentId)
	if !exists {
		return &types.AgentHeartbeatResponse{
			Success: false,
			NextHeartbeat: 0,
		}, nil
	}

	// Update agent info
	agentInfo := map[string]interface{}{
		"agent_id":  req.AgentId,
		"game_id":   req.GameId,
		"env":       req.Env,
		"functions": req.Functions,
		"status":    req.Status,
		"metadata":  req.Metadata,
		"last_heartbeat": time.Now().Unix(),
	}

	l.svcCtx.AgentStore.SetAgentInfo(req.AgentId, agentInfo)

	// Calculate next heartbeat time (based on config)
	nextHeartbeat := time.Now().Add(time.Duration(l.svcCtx.Config.Upstream.HeartbeatInterval) * time.Second).Unix()

	return &types.AgentHeartbeatResponse{
		Success: true,
		NextHeartbeat: nextHeartbeat,
	}, nil
}