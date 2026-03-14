// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type EntityUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新实体
func NewEntityUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityUpdateLogic {
	return &EntityUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityUpdateLogic) EntityUpdate(req *types.EntityUpdateRequest) (*types.EntityUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}
	if req.Data == nil {
		return nil, errors.New("实体数据不能为空")
	}

	entity, err := l.svcCtx.EntityModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.EntityModel.ValidateEntityData(entity.Type, req.Data); err != nil {
		return nil, err
	}

	if err := entity.SetData(req.Data); err != nil {
		return nil, err
	}

	if err := l.svcCtx.EntityModel.Update(l.ctx, id, map[string]interface{}{
		"data": entity.Data,
	}); err != nil {
		return nil, err
	}

	return &types.EntityUpdateResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildEntityDTO(entity),
	}, nil
}
