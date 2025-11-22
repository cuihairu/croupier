package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EdgeHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEdgeHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EdgeHealthLogic {
	return &EdgeHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EdgeHealthLogic) EdgeHealth(req *types.EdgeHealthRequest) (resp *types.EdgeHealthResponse, err error) {
	// TODO: implement the EdgeHealth logic here
	resp = &types.EdgeHealthResponse{
		Status:    "healthy",
		Uptime:    3600,
		Version:   "1.0.0",
		Tunnels:   0,
		Agents:    0,
		Load:      map[string]float64{"cpu": 10.5, "memory": 45.2},
		Timestamp: "2024-01-01T00:00:00Z",
	}
	return
}