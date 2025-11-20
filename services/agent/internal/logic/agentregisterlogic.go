package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AgentRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAgentRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AgentRegisterLogic {
	return &AgentRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AgentRegisterLogic) AgentRegister(req *types.AgentRegisterRequest) (resp *types.AgentRegisterResponse, err error) {
	logx.Infof("Agent registration request: gameId=%s, env=%s, agentId=%s",
		req.GameId, req.Env, req.AgentId)

	// Validate required fields
	if req.AgentId == "" || req.GameId == "" || req.Env == "" {
		return &types.AgentRegisterResponse{
			Success: false,
			Message: "missing required fields: agent_id, game_id, env",
		}, nil
	}

	// Store agent information
	agentInfo := map[string]interface{}{
		"agent_id":  req.AgentId,
		"game_id":   req.GameId,
		"env":       req.Env,
		"rpc_addr":  req.RpcAddr,
		"ip":        req.Ip,
		"type":      req.Type,
		"version":   req.Version,
		"functions": req.Functions,
		"metadata":  req.Metadata,
		"registered_at": time.Now().Unix(),
		"last_heartbeat": time.Now().Unix(),
		"status": "active",
	}

	l.svcCtx.AgentStore.SetAgentInfo(req.AgentId, agentInfo)

	// Generate a simple token (in production, use JWT)
	token := "agent-token-" + req.AgentId + "-" + time.Now().Format("20060102150405")

	logx.Infof("Agent registered successfully: %s", req.AgentId)

	return &types.AgentRegisterResponse{
		Success: true,
		Message: "Agent registered successfully",
		Token:   token,
	}, nil
}