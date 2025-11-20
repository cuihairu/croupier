// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDetailLogic {
	return &ComponentsDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDetailLogic) ComponentsDetail(req *types.ComponentDetailRequest) (*types.ComponentDetailResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("component id required")
	}
	manifest, exists, disabled := l.svcCtx.FindComponent(req.Id)
	if !exists || manifest == nil {
		return nil, errors.New("component not found")
	}

	functions := make([]types.ComponentFunctionInfo, 0, len(manifest.Functions))
	store := l.svcCtx.RegistryStore
	var agents map[string]*registry.AgentSession
	if store != nil {
		store.Mu().RLock()
		agents = make(map[string]*registry.AgentSession, len(store.AgentsUnsafe()))
		for k, v := range store.AgentsUnsafe() {
			agents[k] = v
		}
		store.Mu().RUnlock()
	}
	for _, fn := range manifest.Functions {
		info := types.ComponentFunctionInfo{
			Id:          fn.ID,
			Version:     fn.Version,
			Enabled:     fn.Enabled,
			Description: fn.Description,
		}
		count := 0
		if agents != nil {
			for _, agent := range agents {
				if agent == nil {
					continue
				}
				if meta, ok := agent.Functions[fn.ID]; ok {
					info.Registered = true
					if meta.Enabled {
						info.Enabled = meta.Enabled
					}
					count++
				}
			}
		}
		info.AgentsCount = int64(count)
		functions = append(functions, info)
	}

	files := map[string]int{
		"descriptors": len(manifest.Functions),
		"ui_schemas":  0,
		"entities":    0,
	}

	return &types.ComponentDetailResponse{
		Id:           manifest.ID,
		Name:         manifest.Name,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Category:     manifest.Category,
		Author:       manifest.Author,
		License:      manifest.License,
		Dependencies: manifest.Dependencies,
		Functions:    functions,
		Files:        files,
		SizeBytes:    0,
		Enabled:      !disabled,
	}, nil
}
