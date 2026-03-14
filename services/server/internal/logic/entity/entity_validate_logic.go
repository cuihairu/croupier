// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package entity

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type EntityValidateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 验证实体
func NewEntityValidateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EntityValidateLogic {
	return &EntityValidateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EntityValidateLogic) EntityValidate(req *types.EntityValidateRequest) (*types.EntityValidateResponse, error) {
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

	return &types.EntityValidateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"valid": true,
		},
	}, nil
}
