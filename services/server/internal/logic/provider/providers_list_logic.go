// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ProvidersListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者列表
func NewProvidersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersListLogic {
	return &ProvidersListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersListLogic) ProvidersList(req *types.ProvidersListRequest) (resp *types.ProvidersListResponse, err error) {
	store, err := ensureRegistryStore(l.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	providers := store.ListOpenAPIProviders()
	items := make([]map[string]interface{}, 0, len(providers))
	for _, provider := range providers {
		items = append(items, buildProviderMeta(*provider, false))
	}

	return &types.ProvidersListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
			"page":  req.Page,
			"size":  req.PageSize,
		},
	}, nil
}
