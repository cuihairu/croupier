// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDisableLogic {
	return &ComponentsDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDisableLogic) ComponentsDisable(req *types.ComponentActionRequest) (*types.ComponentUploadResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("component id required")
	}
	cm := l.svcCtx.ComponentManager()
	if cm == nil {
		return nil, errors.New("component manager unavailable")
	}
	if err := cm.DisableComponent(req.Id); err != nil {
		return nil, err
	}
	return &types.ComponentUploadResponse{Ok: true}, nil
}
