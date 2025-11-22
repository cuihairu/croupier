package logic

import (
	"context"

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

func (l *TunnelCreateLogic) TunnelCreate(req *types.TunnelCreateRequest) (resp *types.TunnelCreateResponse, err error) {
	// TODO: implement the TunnelCreate logic here
	resp = &types.TunnelCreateResponse{
		Success:   true,
		TunnelId:  "tunnel-" + req.AgentId + "-" + req.ServerId,
		Message:   "Tunnel created successfully",
		PublicUrl: "https://edge.example.com/proxy/" + "tunnel-" + req.AgentId + "-" + req.ServerId,
	}
	return
}