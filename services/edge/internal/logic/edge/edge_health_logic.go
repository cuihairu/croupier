// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

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
	// todo: add your logic here and delete this line

	return
}
