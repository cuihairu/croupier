// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取组件列表
func NewComponentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsListLogic {
	return &ComponentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsListLogic) ComponentsList(req *types.ComponentsListRequest) (resp *types.ComponentsListResponse, err error) {
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
