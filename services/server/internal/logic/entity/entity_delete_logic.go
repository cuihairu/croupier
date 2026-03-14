// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type EntityDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除实体
func NewEntityDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityDeleteLogic {
	return &EntityDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityDeleteLogic) EntityDelete(req *types.EntityDeleteRequest) (*types.EntityDeleteResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.EntityModel.Delete(l.ctx, id); err != nil {
		return nil, err
	}

	return &types.EntityDeleteResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
