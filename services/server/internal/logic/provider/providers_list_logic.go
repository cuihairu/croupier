// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取提供者列表
func NewProvidersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersListLogic {
	return &ProvidersListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersListLogic) ProvidersList(req *types.ProvidersListRequest) (resp *types.ProvidersListResponse, err error) {
	store, err := ensureRegistryStore(l.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	caps := store.ListProviderCaps()
	items := make([]map[string]interface{}, 0, len(caps))
	for _, cap := range caps {
		items = append(items, buildProviderMeta(cap, false))
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
