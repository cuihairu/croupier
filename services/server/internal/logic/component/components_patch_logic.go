// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package component

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsPatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新组件配置
func NewComponentsPatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsPatchLogic {
	return &ComponentsPatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsPatchLogic) ComponentsPatch(req *types.ComponentPatchRequest) (resp *types.ComponentsPatchResponse, err error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	patchMap, err := normalizePatchMap(req.Patch)
	if err != nil {
		return nil, fmt.Errorf("解析 patch 数据失败: %w", err)
	}

	if len(patchMap) == 0 {
		return &types.ComponentsPatchResponse{
			Code:    0,
			Message: "无需更新",
		}, nil
	}

	var dto componentDTO
	if err := withComponentManagerWrite(l.svcCtx, func(cm *pack.ComponentManager) error {
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		oldCategory := entry.Manifest.Category

		categoryChanged, err := applyComponentPatch(entry.Manifest, patchMap)
		if err != nil {
			return err
		}

		if categoryChanged {
			if err := moveComponentCategory(l.svcCtx.Config, *entry, oldCategory); err != nil {
				return fmt.Errorf("移动组件目录失败: %w", err)
			}
		}

		manifestPath := filepath.Join(componentDir(l.svcCtx.Config, *entry), "manifest.json")
		if err := writeComponentManifest(manifestPath, entry.Manifest); err != nil {
			return fmt.Errorf("写入组件 manifest 失败: %w", err)
		}
		if err := cm.SaveRegistry(); err != nil {
			return fmt.Errorf("保存组件注册表失败: %w", err)
		}

		dto = componentEntryToDTO(l.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &types.ComponentsPatchResponse{
		Code:    0,
		Message: "组件已更新",
		Data:    dto,
	}, nil
}
