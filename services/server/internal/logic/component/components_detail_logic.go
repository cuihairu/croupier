// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ComponentsDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取组件详情
func NewComponentsDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDetailLogic {
	return &ComponentsDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDetailLogic) ComponentsDetail(req *types.ComponentDetailRequest) (resp *types.ComponentsDetailResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看组件详情", "admin:all", "components:read", "components:manage"); err != nil {
		return nil, err
	}

	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto componentDTO
	if err := withComponentManagerRead(l.svcCtx, func(cm *pack.ComponentManager) error {
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(l.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &types.ComponentsDetailResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"component": dto,
		},
	}, nil
}
