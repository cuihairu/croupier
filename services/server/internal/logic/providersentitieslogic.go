// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProvidersEntitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvidersEntitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvidersEntitiesLogic {
	return &ProvidersEntitiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProvidersEntitiesLogic) ProvidersEntities() (*types.ProvidersEntitiesResponse, error) {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return nil, errRegistryUnavailable
	}
	caps := store.ListProviderCaps()
	out := make([]types.ProviderEntitySet, 0, len(caps))
	for _, cap := range caps {
		var manifest struct {
			Entities []interface{} `json:"entities"`
		}
		if err := json.Unmarshal(cap.Manifest, &manifest); err != nil || len(manifest.Entities) == 0 {
			continue
		}
		out = append(out, types.ProviderEntitySet{
			Provider: types.ProviderReference{
				Id:      cap.ID,
				Version: cap.Version,
			},
			Entities: manifest.Entities,
		})
	}
	return &types.ProvidersEntitiesResponse{Providers: out}, nil
}
