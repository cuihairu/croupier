// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PacksReloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPacksReloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PacksReloadLogic {
	return &PacksReloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PacksReloadLogic) PacksReload() (*types.PacksReloadResponse, error) {
	l.svcCtx.ReloadDescriptors()
	return &types.PacksReloadResponse{Ok: true}, nil
}
