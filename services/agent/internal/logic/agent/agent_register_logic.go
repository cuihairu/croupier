// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func (l *AgentRegisterLogic) AgentRegister(req *types.AgentRegisterRequest) (*types.AgentRegisterResponse, error) {
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

	state.Agents[agentID] = &svc.AgentRecord{
		ID:            agentID,
		GameID:        strings.TrimSpace(req.GameId),
		Env:           strings.TrimSpace(req.Env),
		Type:          strings.TrimSpace(req.Type),
		Version:       strings.TrimSpace(req.Version),
		RPCAddr:       strings.TrimSpace(req.RpcAddr),
		Status:        "registered",
		Functions:     req.Functions,
		Metadata:      svc.CloneStringMap(req.Metadata),
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	token := fmt.Sprintf("%s:%d", agentID, time.Now().Unix())
	l.Infof("注册代理成功: %s (%s/%s)", agentID, req.GameId, req.Env)

	return &types.AgentRegisterResponse{
		Success: true,
		Message: "agent registered",
		Token:   token,
	}, nil
}
