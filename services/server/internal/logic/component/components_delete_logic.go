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

type ComponentsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除组件
func NewComponentsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsDeleteLogic {
	return &ComponentsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsDeleteLogic) ComponentsDelete(req *types.ComponentActionRequest) (resp *types.ComponentsDeleteResponse, err error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var removed componentDTO
	if err := withComponentManagerWrite(l.svcCtx, func(cm *pack.ComponentManager) error {
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		removed = componentEntryToDTO(l.svcCtx.Config, *entry)

		if entry.Status == componentStatusInstalled {
			if err := cm.UninstallComponent(req.ID); err != nil {
				return fmt.Errorf("卸载组件失败: %w", err)
			}
			return nil
		}

		if err := removeDisabledComponent(cm, l.svcCtx.Config, *entry); err != nil {
			return fmt.Errorf("删除禁用组件失败: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &types.ComponentsDeleteResponse{
		Code:    0,
		Message: "组件已删除",
		Data:    removed,
	}, nil
}
