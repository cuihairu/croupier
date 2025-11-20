// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDeleteLogic {
	return &ComponentsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDeleteLogic) ComponentsDelete(req *types.ComponentActionRequest) (*types.ComponentUploadResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("component id required")
	}
	cm := l.svcCtx.ComponentManager()
	if cm == nil {
		return nil, errors.New("component manager unavailable")
	}
	if err := cm.UninstallComponent(req.Id); err != nil {
		return nil, err
	}
	return &types.ComponentUploadResponse{Ok: true}, nil
}
