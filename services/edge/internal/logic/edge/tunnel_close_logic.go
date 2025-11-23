// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

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
	// todo: add your logic here and delete this line

	return
}
