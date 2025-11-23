// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersEntitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者实体
func NewProvidersEntitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersEntitiesLogic {
	return &ProvidersEntitiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersEntitiesLogic) ProvidersEntities(req *types.ProvidersEntitiesRequest) (resp *types.ProvidersEntitiesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
