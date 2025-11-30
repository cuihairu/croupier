// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelCreateLogic {
	return &TunnelCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelCreateLogic) TunnelCreate(req *types.TunnelCreateRequest) (*types.TunnelCreateResponse, error) {
	if req == nil {
		return nil, errors.New("missing payload")
	}
	if strings.TrimSpace(req.AgentId) == "" || strings.TrimSpace(req.RemoteAddr) == "" {
		return nil, errors.New("agent_id 与 remote_addr 不能为空")
	}

	id := generateTunnelID(req.AgentId)
	now := time.Now()

	state := l.svcCtx.State
	state.Mu.Lock()
	state.Tunnels[id] = &svc.TunnelRecord{
		ID:         id,
		AgentID:    strings.TrimSpace(req.AgentId),
		ServerID:   strings.TrimSpace(req.ServerId),
		Protocol:   defaultString(req.Protocol, "http"),
		RemoteAddr: strings.TrimSpace(req.RemoteAddr),
		LocalAddr:  strings.TrimSpace(req.LocalAddr),
		Status:     "active",
		Options:    svc.CloneMap(req.Options),
		CreatedAt:  now,
		LastActive: now,
	}
	state.Mu.Unlock()

	publicHost := strings.TrimSpace(l.svcCtx.Config.Server.PublicAddr)
	if publicHost == "" {
		publicHost = "edge.local"
	}
	publicURL := fmt.Sprintf("https://%s/proxy/%s", publicHost, id)

	return &types.TunnelCreateResponse{
		Success:   true,
		TunnelId:  id,
		Message:   "tunnel created",
		PublicUrl: publicURL,
	}, nil
}
