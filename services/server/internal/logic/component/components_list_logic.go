// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ComponentsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取组件列表
func NewComponentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsListLogic {
	return &ComponentsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsListLogic) ComponentsList(req *types.ComponentsListRequest) (resp *types.ComponentsListResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看组件列表", "admin:all", "components:read", "components:manage"); err != nil {
		return nil, err
	}

	page := 1
	if req != nil && req.Page > 0 {
		page = req.Page
	}
	pageSize := 20
	if req != nil && req.PageSize > 0 {
		pageSize = req.PageSize
	}

	var (
		items          []componentDTO
		installedCount int
		disabledCount  int
		functionsCount int
	)

	if err := withComponentManagerRead(l.svcCtx, func(cm *pack.ComponentManager) error {
		items, installedCount, disabledCount, functionsCount = buildComponentList(l.svcCtx.Config, cm)
		return nil
	}); err != nil {
		return nil, err
	}

	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	data := map[string]interface{}{
		"items": items[start:end],
		"total": total,
		"page":  page,
		"size":  pageSize,
		"counts": map[string]int{
			"installed": installedCount,
			"disabled":  disabledCount,
			"functions": functionsCount,
		},
	}

	return &types.ComponentsListResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}
