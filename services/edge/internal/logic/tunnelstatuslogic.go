package logic

import (
	"context"

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
	// TODO: implement the TunnelStatus logic here
	resp = &types.TunnelStatusResponse{
		TunnelId:    req.TunnelId,
		Status:      "active",
		Protocol:    "http",
		RemoteAddr:  "https://server.example.com",
		LocalAddr:   "http://localhost:8080",
		Connections: 3,
		BytesIn:     1024000,
		BytesOut:    2048000,
		CreatedAt:   "2024-01-01T00:00:00Z",
		LastActive:  "2024-01-01T01:00:00Z",
	}
	return
}