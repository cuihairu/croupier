// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type EntityDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取实体详情
func NewEntityDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityDetailLogic {
	return &EntityDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityDetailLogic) EntityDetail(req *types.EntityDetailRequest) (*types.EntityDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	entity, err := l.svcCtx.EntityModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.EntityDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildEntityDTO(entity),
	}, nil
}
