// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsEnableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 启用组件
func NewComponentsEnableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsEnableLogic {
	return &ComponentsEnableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsEnableLogic) ComponentsEnable(req *types.ComponentActionRequest) (resp *types.ComponentsEnableResponse, err error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto componentDTO
	if err := withComponentManagerWrite(l.svcCtx, func(cm *pack.ComponentManager) error {
		if err := cm.EnableComponent(req.ID); err != nil {
			return fmt.Errorf("启用组件失败: %w", err)
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

	return &types.ComponentsEnableResponse{
		Code:    0,
		Message: "组件已启用",
		Data:    dto,
	}, nil
}
