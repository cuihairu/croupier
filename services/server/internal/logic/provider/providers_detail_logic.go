// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者详情
func NewProvidersDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDetailLogic {
	return &ProvidersDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDetailLogic) ProvidersDetail(req *types.ProviderDetailRequest) (resp *types.ProviderDetailResponse, err error) {
	caps, err := getProviderCaps(l.svcCtx.RegistryStore, req.ID)
	if err != nil {
		return nil, err
	}

	meta := buildProviderMeta(caps, true)

	return &types.ProviderDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    meta,
	}, nil
}
