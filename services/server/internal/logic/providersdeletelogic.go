// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvidersDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDeleteLogic {
	return &ProvidersDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDeleteLogic) ProvidersDelete(req *types.ProviderActionRequest) (resp *types.ProviderDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
