// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EntityCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建实体
func NewEntityCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityCreateLogic {
	return &EntityCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityCreateLogic) EntityCreate(req *types.EntityCreateRequest) (*types.EntityCreateResponse, error) {
	entityType := strings.TrimSpace(req.Type)
	if entityType == "" {
		return nil, errors.New("实体类型不能为空")
	}
	if req.Data == nil {
		return nil, errors.New("实体数据不能为空")
	}

	if err := l.svcCtx.EntityModel.ValidateEntityData(entityType, req.Data); err != nil {
		return nil, err
	}

	entity := &model.Entity{
		Type:   entityType,
		Status: 1,
	}
	if err := entity.SetData(req.Data); err != nil {
		return nil, err
	}

	if err := l.svcCtx.EntityModel.Create(l.ctx, entity); err != nil {
		return nil, err
	}

	return &types.EntityCreateResponse{
		Code:    0,
		Message: "OK",
		Data:    utils.BuildEntityDTO(entity),
	}, nil
}
