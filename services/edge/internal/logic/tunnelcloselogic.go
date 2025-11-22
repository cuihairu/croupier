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
	// TODO: implement the TunnelClose logic here
	resp = &types.TunnelCloseResponse{
		Success: true,
		Message: "Tunnel closed successfully",
	}
	return
}