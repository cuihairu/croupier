// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersCapabilitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者能力
func NewProvidersCapabilitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersCapabilitiesLogic {
	return &ProvidersCapabilitiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersCapabilitiesLogic) ProvidersCapabilities(req *types.ProvidersCapabilitiesRequest) (resp *types.ProvidersCapabilitiesResponse, err error) {
	store, err := ensureRegistryStore(l.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}
	providers := store.ListOpenAPIProviders()

	items := make([]map[string]interface{}, 0, len(providers))
	for _, provider := range providers {
		items = append(items, buildProviderMeta(*provider, true))
	}

	return &types.ProvidersCapabilitiesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": len(items),
		},
	}, nil
}
