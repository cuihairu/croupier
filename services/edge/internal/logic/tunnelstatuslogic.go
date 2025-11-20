package logic

import (
	"context"
	"fmt"
	"time"

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

func (l *TunnelStatusLogic) TunnelStatus(req *types.TunnelStatusRequest) (resp *types.TunnelStatusResponse, err error) {
	logx.Infof("Tunnel status request: tunnelId=%s", req.TunnelId)

	// Get tunnel from manager
	tunnel, exists := l.svcCtx.TunnelMgr.GetTunnel(req.TunnelId)
	if !exists {
		return &types.TunnelStatusResponse{
			TunnelId:   req.TunnelId,
			Status:     "not_found",
			Protocol:   "",
			RemoteAddr: "",
			LocalAddr:  "",
			CreatedAt:  "",
			LastActive: "",
		}, nil
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
		CreatedAt:   tunnel.CreatedAt.Format(time.RFC3339),
		LastActive:  tunnel.LastActive.Format(time.RFC3339),
	}, nil
}