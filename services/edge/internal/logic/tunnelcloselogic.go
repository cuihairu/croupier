package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelCloseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelCloseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelCloseLogic {
	return &TunnelCloseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelCloseLogic) TunnelClose(req *types.TunnelCloseRequest) (resp *types.TunnelCloseResponse, err error) {
	logx.Infof("Tunnel close request: tunnelId=%s", req.TunnelId)

	// Remove from load balancer first
	l.svcCtx.LoadBalancer.RemoveTunnel(req.TunnelId)

	// Close tunnel
	success := l.svcCtx.TunnelMgr.CloseTunnel(req.TunnelId)
	if !success {
		return &types.TunnelCloseResponse{
			Success: false,
			Message: "tunnel not found or already closed",
		}, nil
	}

	logx.Infof("Tunnel closed successfully: %s", req.TunnelId)

	return &types.TunnelCloseResponse{
		Success: true,
		Message: "Tunnel closed successfully",
	}, nil
}