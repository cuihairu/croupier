package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelListLogic {
	return &TunnelListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelListLogic) TunnelList(req *types.TunnelListRequest) (resp *types.TunnelListResponse, err error) {
	// TODO: implement the TunnelList logic here
	resp = &types.TunnelListResponse{
		Tunnels: []types.TunnelStatusResponse{
			{
				TunnelId:    "tunnel-1",
				Status:      "active",
				Protocol:    "http",
				RemoteAddr:  "https://server1.example.com",
				LocalAddr:   "http://localhost:8080",
				Connections: 3,
				BytesIn:     1024000,
				BytesOut:    2048000,
				CreatedAt:   "2024-01-01T00:00:00Z",
				LastActive:  "2024-01-01T01:00:00Z",
			},
		},
		Total: 1,
		Page:  req.Page,
		Size:  req.Size,
	}
	return
}