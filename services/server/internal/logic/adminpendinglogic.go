package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminPendingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminPendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPendingLogic {
	return &AdminPendingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPendingLogic) AdminPending() (*types.AdminPendingResponse, error) {
	overrides := l.svcCtx.LoadUIOverrides()
	meta := l.svcCtx.RegistryStore.BuildFunctionIndex()
	pending := []types.AdminPendingFunction{}
	seen := map[string]struct{}{}

	for fid, data := range meta {
		if _, ok := overrides[fid]; ok {
			continue
		}
		item := buildPendingItem(fid, data)
		pending = append(pending, item)
		seen[fid] = struct{}{}
	}

	store := l.svcCtx.RegistryStore
	if store != nil {
		store.Mu().RLock()
		for _, agent := range store.AgentsUnsafe() {
			for fid := range agent.Functions {
				if strings.TrimSpace(fid) == "" {
					continue
				}
				if _, ok := seen[fid]; ok {
					continue
				}
				if _, ok := overrides[fid]; ok {
					continue
				}
				var data map[string]interface{}
				if m, ok := meta[fid]; ok {
					data = m
				}
				item := buildPendingItem(fid, data)
				pending = append(pending, item)
				seen[fid] = struct{}{}
			}
		}
		store.Mu().RUnlock()
	}

	return &types.AdminPendingResponse{Pending: pending}, nil
}

func buildPendingItem(fid string, data map[string]interface{}) types.AdminPendingFunction {
	item := types.AdminPendingFunction{FunctionId: fid}
	if data == nil {
		return item
	}
	if dn, ok := data["display_name"]; ok {
		item.DisplayName = dn
	}
	if sm, ok := data["summary"]; ok {
		item.Summary = sm
	}
	if perm, ok := data["permissions"]; ok {
		item.SuggestedPermissions = perm
	}
	return item
}
