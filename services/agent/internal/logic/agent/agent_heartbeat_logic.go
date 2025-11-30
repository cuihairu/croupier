// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"strings"
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

func (l *AgentHeartbeatLogic) AgentHeartbeat(req *types.AgentHeartbeatRequest) (*types.AgentHeartbeatResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}
	agentID := strings.TrimSpace(req.AgentId)
	if agentID == "" {
		return nil, errors.New("agent_id 不能为空")
	}

	state := l.svcCtx.AgentState
	state.Mu.Lock()
	defer state.Mu.Unlock()

	record := state.Agents[agentID]
	if record == nil {
		record = &svc.AgentRecord{
			ID:           agentID,
			GameID:       strings.TrimSpace(req.GameId),
			Env:          strings.TrimSpace(req.Env),
			RegisteredAt: time.Now(),
		}
		state.Agents[agentID] = record
	}

	record.Status = defaultString(req.Status, "healthy")
	record.Functions = req.Functions
	record.Metadata = svc.CloneStringMap(req.Metadata)
	record.LastHeartbeat = time.Now()

	next := l.svcCtx.Config.Upstream.HeartbeatInterval
	if next <= 0 {
		next = 30
	}

	return &types.AgentHeartbeatResponse{
		Success:       true,
		NextHeartbeat: next,
	}, nil
}
