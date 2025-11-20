// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersReloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvidersReloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersReloadLogic {
	return &ProvidersReloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersReloadLogic) ProvidersReload(req *types.ProviderActionRequest) (resp *types.ProviderReloadResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
