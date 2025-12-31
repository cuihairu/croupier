// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsInstallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 安装组件
func NewComponentsInstallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsInstallLogic {
	return &ComponentsInstallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsInstallLogic) ComponentsInstall(req *types.ComponentsInstallRequest) (resp *types.ComponentsInstallResponse, err error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权安装组件", "admin:all", "components:install", "components:manage"); err != nil {
		return nil, err
	}

	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("组件名称不能为空")
	}

	var dto componentDTO
	if err := withComponentManagerWrite(l.svcCtx, func(cm *pack.ComponentManager) error {
		source, err := locateComponentSource(l.svcCtx.Config, req.Name, req.Version)
		if err != nil {
			return err
		}
		manifest, err := readComponentManifest(source)
		if err != nil {
			return fmt.Errorf("读取组件定义失败: %w", err)
		}
		if strings.TrimSpace(manifest.ID) == "" {
			return errors.New("组件 manifest 缺少 ID")
		}
		if _, exists := cm.ListInstalled()[manifest.ID]; exists {
			return fmt.Errorf("组件 %s 已安装", manifest.ID)
		}
		if _, exists := cm.ListDisabled()[manifest.ID]; exists {
			return fmt.Errorf("组件 %s 当前处于禁用状态，请先删除或启用", manifest.ID)
		}
		if err := cm.InstallComponent(source); err != nil {
			return fmt.Errorf("安装组件失败: %w", err)
		}
		entry, err := findComponentEntry(cm, manifest.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(l.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &types.ComponentsInstallResponse{
		Code:    0,
		Message: "组件安装完成",
		Data:    dto,
	}, nil
}
