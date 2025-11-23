// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

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
	// todo: add your logic here and delete this line

	return
}
