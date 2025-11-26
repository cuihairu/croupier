// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersDescriptorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者描述符
func NewProvidersDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDescriptorsLogic {
	return &ProvidersDescriptorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDescriptorsLogic) ProvidersDescriptors(req *types.ProvidersDescriptorsRequest) (resp *types.ProvidersDescriptorsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
