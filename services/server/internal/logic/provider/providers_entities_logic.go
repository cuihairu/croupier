// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package provider

import (
	"context"
	"strings"

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
	store, err := ensureRegistryStore(l.svcCtx.RegistryStore)
	if err != nil {
		return nil, err
	}

	var entities []map[string]interface{}
	if strings.TrimSpace(req.ID) == "" || req.ID == "*" {
		entities = aggregateEntities(store)
	} else {
		entities, err = aggregateEntitiesForProvider(store, req.ID)
		if err != nil {
			return nil, err
		}
	}

	return &types.ProvidersEntitiesResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": entities,
			"total": len(entities),
		},
	}, nil
}
