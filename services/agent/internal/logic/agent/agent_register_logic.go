// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentcore "github.com/cuihairu/croupier/internal/app/agent"
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

	if l.svcCtx.Core == nil {
		return nil, errors.New("agent core is not running")
	}

	configuredID := strings.TrimSpace(l.svcCtx.Config.Agent.ID)
	agentID := strings.TrimSpace(req.AgentId)
	if agentID == "" {
		agentID = configuredID
	}
	if agentID == "" {
		return nil, errors.New("agent_id 不能为空")
	}
	if configuredID != "" && agentID != configuredID {
		return nil, errors.New("agent_id mismatch")
	}

	meta := agentcore.UpstreamMetadata{
		GameID:  defaultString(strings.TrimSpace(req.GameId), strings.TrimSpace(l.svcCtx.Config.Agent.GameID)),
		Env:     defaultString(strings.TrimSpace(req.Env), strings.TrimSpace(l.svcCtx.Config.Agent.Env)),
		Version: defaultString(strings.TrimSpace(req.Version), ""),
		RPCAddr: defaultString(strings.TrimSpace(req.RpcAddr), strings.TrimSpace(l.svcCtx.Config.Agent.LocalAddr)),
	}
	l.svcCtx.Core.WithUpstreamMetadata(meta)

	syncCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()
	if err := l.svcCtx.Core.SyncUpstream(syncCtx); err != nil {
		l.Errorf("upstream sync skipped/failed: %v", err)
	}

	token := fmt.Sprintf("%s:%d", agentID, time.Now().Unix())
	l.Infof("注册代理成功: %s (%s/%s)", agentID, req.GameId, req.Env)

	// Set registration timestamp
	l.svcCtx.SetRegisteredAt(time.Now())

	return &types.AgentRegisterResponse{
		Success: true,
		Message: "agent registered",
		Token:   token,
	}, nil
}
