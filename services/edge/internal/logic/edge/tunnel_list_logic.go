// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

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
	// todo: add your logic here and delete this line

	return
}
