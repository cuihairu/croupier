// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntitiesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实体列表
func NewEntitiesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntitiesListLogic {
	return &EntitiesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntitiesListLogic) EntitiesList(req *types.EntitiesListRequest) (*types.EntitiesListResponse, error) {
	opts := model.ListEntitiesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Type:     strings.TrimSpace(req.Type),
	}

	entities, total, err := l.svcCtx.EntityModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(entities))
	for i := range entities {
		items = append(items, utils.BuildEntityDTO(&entities[i]))
	}

	return &types.EntitiesListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"total": total,
			"page":  opts.Page,
			"size":  opts.PageSize,
		},
	}, nil
}
