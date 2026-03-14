// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type EntityPreviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览实体
func NewEntityPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityPreviewLogic {
	return &EntityPreviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityPreviewLogic) EntityPreview(req *types.EntityPreviewRequest) (*types.EntityPreviewResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	entity, err := l.svcCtx.EntityModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.EntityPreviewResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"data": utils.BuildEntityDTO(entity)["data"],
		},
	}, nil
}
