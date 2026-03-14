// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ProvidersDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除提供者
func NewProvidersDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersDeleteLogic {
	return &ProvidersDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersDeleteLogic) ProvidersDelete(req *types.ProviderActionRequest) (resp *types.ProviderDeleteResponse, err error) {
	if err := deleteProviderCaps(l.svcCtx.RegistryStore, req.ID); err != nil {
		return nil, err
	}

	return &types.ProviderDeleteResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id": req.ID,
		},
	}, nil
}
