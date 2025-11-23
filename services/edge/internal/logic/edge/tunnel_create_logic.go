// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

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
	// todo: add your logic here and delete this line

	return
}
