package component

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns a list of components
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看组件列表", "admin:all", "components:read", "components:manage"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize

	var (
		items          []Component
		installedCount int
		disabledCount  int
		functionsCount int
	)

	if err := withComponentManagerRead(s.svcCtx, func(cm *pack.ComponentManager) error {
		items, installedCount, disabledCount, functionsCount = buildComponentList(s.svcCtx.Config, cm)
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

	return &ListResponse{
		Items: items[start:end],
		Total: total,
		Page:  page,
		Size:  pageSize,
		Counts: map[string]int{
			"installed": installedCount,
			"disabled":  disabledCount,
			"functions": functionsCount,
		},
	}, nil
}

// Install installs a component
func (s *Service) Install(ctx context.Context, req *InstallRequest) (*InstallResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权安装组件", "admin:all", "components:install", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("组件名称不能为空")
	}

	var dto Component
	if err := withComponentManagerWrite(s.svcCtx, func(cm *pack.ComponentManager) error {
		source, err := locateComponentSource(s.svcCtx.Config, req.Name, req.Version)
		if err != nil {
			return err
		}
		manifest, err := readComponentManifest(source)
		if err != nil {
			return errorx.NewBadRequest("读取组件定义失败")
		}
		if strings.TrimSpace(manifest.ID) == "" {
			return errors.New("组件 manifest 缺少 ID")
		}
		if _, exists := cm.ListInstalled()[manifest.ID]; exists {
			return errorx.NewConflict("组件已安装: " + manifest.ID)
		}
		if _, exists := cm.ListDisabled()[manifest.ID]; exists {
			return errorx.NewConflict("组件当前处于禁用状态，请先删除或启用: " + manifest.ID)
		}
		if err := cm.InstallComponent(source); err != nil {
			return errorx.NewInternalError("安装组件失败")
		}
		entry, err := findComponentEntry(cm, manifest.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(s.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &InstallResponse{
		Component: dto,
	}, nil
}

// Get returns component details
func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看组件详情", "admin:all", "components:read", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto Component
	if err := withComponentManagerRead(s.svcCtx, func(cm *pack.ComponentManager) error {
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(s.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &GetResponse{
		Component: dto,
	}, nil
}

// Enable enables a component
func (s *Service) Enable(ctx context.Context, req *EnableRequest) (*EnableResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权启用组件", "admin:all", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto Component
	if err := withComponentManagerWrite(s.svcCtx, func(cm *pack.ComponentManager) error {
		if err := cm.EnableComponent(req.ID); err != nil {
			return errorx.NewInternalError("启用组件失败")
		}
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(s.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &EnableResponse{
		Component: dto,
	}, nil
}

// Disable disables a component
func (s *Service) Disable(ctx context.Context, req *DisableRequest) (*DisableResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权禁用组件", "admin:all", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var dto Component
	if err := withComponentManagerWrite(s.svcCtx, func(cm *pack.ComponentManager) error {
		if err := cm.DisableComponent(req.ID); err != nil {
			return errorx.NewInternalError("禁用组件失败")
		}
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		dto = componentEntryToDTO(s.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &DisableResponse{
		Component: dto,
	}, nil
}

// Delete deletes a component
func (s *Service) Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除组件", "admin:all", "components:uninstall", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	var removed Component
	if err := withComponentManagerWrite(s.svcCtx, func(cm *pack.ComponentManager) error {
		entry, err := findComponentEntry(cm, req.ID)
		if err != nil {
			return err
		}
		removed = componentEntryToDTO(s.svcCtx.Config, *entry)

		if entry.Status == componentStatusInstalled {
			if err := cm.UninstallComponent(req.ID); err != nil {
				return errorx.NewInternalError("卸载组件失败")
			}
			return nil
		}

		if err := removeDisabledComponent(cm, s.svcCtx.Config, *entry); err != nil {
			return errorx.NewInternalError("删除禁用组件失败")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &DeleteResponse{
		Component: removed,
	}, nil
}

// Patch updates a component configuration
func (s *Service) Patch(ctx context.Context, req *PatchRequest) (*PatchResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权更新组件配置", "admin:all", "components:manage"); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("组件ID不能为空")
	}

	patchMap, err := normalizePatchMap(req.Patch)
	if err != nil {
		return nil, errorx.NewBadRequest("解析 patch 数据失败")
	}

	if len(patchMap) == 0 {
		return &PatchResponse{
			Component: Component{},
		}, nil
	}

	var dto Component
	if err := withComponentManagerWrite(s.svcCtx, func(cm *pack.ComponentManager) error {
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
			if err := moveComponentCategory(s.svcCtx.Config, *entry, oldCategory); err != nil {
				return errorx.NewInternalError("移动组件目录失败")
			}
		}

		manifestPath := filepath.Join(componentDir(s.svcCtx.Config, *entry), "manifest.json")
		if err := writeComponentManifest(manifestPath, entry.Manifest); err != nil {
			return errorx.NewInternalError("写入组件 manifest 失败")
		}
		if err := cm.SaveRegistry(); err != nil {
			return errorx.NewInternalError("保存组件注册表失败")
		}

		dto = componentEntryToDTO(s.svcCtx.Config, *entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return &PatchResponse{
		Component: dto,
	}, nil
}
