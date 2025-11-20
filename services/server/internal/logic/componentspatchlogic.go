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

type ComponentsPatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsPatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsPatchLogic {
	return &ComponentsPatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsPatchLogic) ComponentsPatch(req *types.ComponentPatchRequest) (*types.ComponentPatchResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("component id required")
	}
	_, exists, _ := l.svcCtx.FindComponent(req.Id)
	if !exists {
		return nil, errors.New("component not found")
	}
	cm := l.svcCtx.ComponentManager()
	if cm == nil {
		return nil, errors.New("component manager unavailable")
	}
	updated := make([]string, 0, 2)
	if req.Enabled != nil {
		if *req.Enabled {
			if err := cm.EnableComponent(req.Id); err != nil {
				return nil, err
			}
		} else {
			if err := cm.DisableComponent(req.Id); err != nil {
				return nil, err
			}
		}
		updated = append(updated, "component_enabled")
	}
	if len(req.Functions) > 0 {
		for funcID, cfg := range req.Functions {
			logx.WithContext(l.ctx).Infof("component %s function %s config %#v", req.Id, funcID, cfg)
		}
		updated = append(updated, "function_configs")
	}
	return &types.ComponentPatchResponse{
		Ok:      true,
		Updated: updated,
	}, nil
}
