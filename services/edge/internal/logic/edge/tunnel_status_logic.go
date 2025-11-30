// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelStatusLogic {
	return &TunnelStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelStatusLogic) TunnelStatus(req *types.TunnelStatusRequest) (*types.TunnelStatusResponse, error) {
	if req == nil || strings.TrimSpace(req.TunnelId) == "" {
		return nil, errors.New("tunnel_id 不能为空")
	}

	state := l.svcCtx.State
	state.Mu.RLock()
	tunnel := state.Tunnels[req.TunnelId]
	state.Mu.RUnlock()

	if tunnel == nil {
		return nil, errors.New("tunnel not found")
	}

	return &types.TunnelStatusResponse{
		TunnelId:    tunnel.ID,
		Status:      tunnel.Status,
		Protocol:    tunnel.Protocol,
		RemoteAddr:  tunnel.RemoteAddr,
		LocalAddr:   tunnel.LocalAddr,
		Connections: tunnel.Connections,
		BytesIn:     tunnel.BytesIn,
		BytesOut:    tunnel.BytesOut,
		CreatedAt:   formatTime(tunnel.CreatedAt),
		LastActive:  formatTime(tunnel.LastActive),
	}, nil
}
