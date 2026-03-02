// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsDisableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 禁用组件
func NewComponentsDisableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDisableLogic {
	return &ComponentsDisableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDisableLogic) ComponentsDisable(req *types.ComponentActionRequest) (resp *types.ComponentsDisableResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权禁用组件", "admin:all", "components:manage"); err != nil {
		return nil, err
	}

	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto componentDTO
	if err := withComponentManagerWrite(l.svcCtx, func(cm *pack.ComponentManager) error {
		if err := cm.DisableComponent(req.ID); err != nil {
			return errorx.NewInternalError("禁用组件失败")
		}
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(l.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &types.ComponentsDisableResponse{
		Code:    0,
		Message: "组件已禁用",
		Data:    dto,
	}, nil
}
