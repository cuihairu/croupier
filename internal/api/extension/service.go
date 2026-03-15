package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) CatalogList(ctx context.Context, req ExtensionCatalogListRequest) (*ExtensionCatalogListResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展目录"); err != nil {
		return nil, err
	}
	items, total, err := s.svcCtx.Extensions.Catalog.List(ctx, catalogListQuery(req))
	if err != nil {
		return nil, mapServiceError(err)
	}
	installedSet, err := s.activeInstalledExtensionSet(ctx)
	if err != nil {
		return nil, err
	}
	respItems := make([]ExtensionCatalogItem, 0, len(items))
	for _, item := range items {
		installed := installedSet[normalizeExtensionID(item.ExtensionID)]
		defaultInstall, tags := s.resolveCatalogMetadata(ctx, item.ExtensionID, item.LatestVersion)
		respItems = append(respItems, ExtensionCatalogItem{
			ID:             item.ExtensionID,
			Name:           item.Name,
			DisplayName:    item.DisplayName,
			Vendor:         item.Vendor,
			Kind:           item.Kind,
			Summary:        item.Summary,
			IconURL:        item.IconURL,
			Status:         item.Status,
			LatestVersion:  item.LatestVersion,
			Installed:      installed,
			DefaultInstall: defaultInstall,
			Tags:           tags,
		})
	}
	return &ExtensionCatalogListResponse{Code: 200, Message: "success", Total: total, Items: respItems}, nil
}

func (s *Service) CatalogDetail(ctx context.Context, extensionID string) (*ExtensionCatalogDetailResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展详情"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(extensionID) == "" {
		return nil, errorx.NewBadRequest("extension id is required")
	}
	item, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	activeInstalled, err := s.findActiveInstallationByExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	releaseItems := make([]ExtensionReleaseItem, 0, len(releases))
	for _, r := range releases {
		releaseItems = append(releaseItems, ExtensionReleaseItem{
			Version:        r.Version,
			ReleaseChannel: r.ReleaseChannel,
			MinCoreVersion: r.MinCoreVersion,
			PublishedAt:    r.PublishedAtUnix,
			Changelog:      r.Changelog,
		})
	}
	manifest := s.svcCtx.Extensions.Manifest.MustJSON(firstManifest(releases))
	defaultInstall := extractDefaultInstall(manifest)
	tags := extractTags(manifest)
	return &ExtensionCatalogDetailResponse{
		Code:    200,
		Message: "success",
		Item: &ExtensionCatalogItem{
			ID:             item.ExtensionID,
			Name:           item.Name,
			DisplayName:    item.DisplayName,
			Vendor:         item.Vendor,
			Kind:           item.Kind,
			Summary:        item.Summary,
			IconURL:        item.IconURL,
			Status:         item.Status,
			LatestVersion:  item.LatestVersion,
			Installed:      activeInstalled != nil,
			DefaultInstall: defaultInstall,
			Tags:           tags,
		},
		Releases:     releaseItems,
		Manifest:     manifest,
		Capabilities: extractCapabilities(manifest),
	}, nil
}

func (s *Service) CatalogReleases(ctx context.Context, extensionID string) (*ExtensionCatalogReleasesResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展版本列表"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(extensionID) == "" {
		return nil, errorx.NewBadRequest("extension id is required")
	}
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	items := make([]ExtensionReleaseItem, 0, len(releases))
	for _, r := range releases {
		items = append(items, ExtensionReleaseItem{
			Version:        r.Version,
			ReleaseChannel: r.ReleaseChannel,
			MinCoreVersion: r.MinCoreVersion,
			PublishedAt:    r.PublishedAtUnix,
			Changelog:      r.Changelog,
		})
	}
	return &ExtensionCatalogReleasesResponse{
		Code:     200,
		Message:  "success",
		Total:    int64(len(items)),
		Releases: items,
	}, nil
}

func (s *Service) Install(ctx context.Context, req ExtensionInstallRequest, operator string) (*ExtensionInstallResponse, error) {
	if err := s.requireWritePermission(ctx, "无权安装扩展"); err != nil {
		return nil, err
	}
	if err := s.validateDependencies(ctx, req.ExtensionID, req.ReleaseVersion); err != nil {
		return nil, err
	}
	conflict, existing, err := s.findInstallationConflict(ctx, req)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, errorx.NewConflictWithDetails(
			"extension already installed for same scope/target",
			map[string]any{
				"code":            "extension_already_installed",
				"installation_id": existing.ID,
				"extension_id":    existing.ExtensionID,
				"scope_type":      existing.ScopeType,
				"scope_id":        existing.ScopeID,
				"target_type":     existing.TargetType,
				"target_id":       existing.TargetID,
				"release_version": existing.ReleaseVersion,
			},
		)
	}
	if err := s.validateInstallConfig(ctx, req.ExtensionID, req.ReleaseVersion, req.Config); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Install(ctx, extensioninstallation.InstallRequest{
		ExtensionID:    req.ExtensionID,
		ReleaseVersion: req.ReleaseVersion,
		ScopeType:      req.ScopeType,
		ScopeID:        req.ScopeID,
		TargetType:     req.TargetType,
		TargetID:       req.TargetID,
		Config:         req.Config,
		SecretRefs:     req.SecretRefs,
		Operator:       operator,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionInstallResponse{Code: 200, Message: "success", InstallationID: item.ID, Status: item.Status}, nil
}

func (s *Service) InstallationList(ctx context.Context, req ExtensionInstallationListRequest) (*ExtensionInstallationListResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展安装实例"); err != nil {
		return nil, err
	}
	items, total, err := s.svcCtx.Extensions.Installation.List(ctx, installationListQuery(req))
	if err != nil {
		return nil, mapServiceError(err)
	}
	respItems := make([]ExtensionInstallationItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toInstallationItem(item))
	}
	return &ExtensionInstallationListResponse{Code: 200, Message: "success", Total: total, Items: respItems}, nil
}

func (s *Service) InstallationDetail(ctx context.Context, id uint) (*ExtensionInstallationDetailResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展安装详情"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	events, _, err := s.svcCtx.Extensions.Installation.ListEvents(ctx, id, extensioninstallation.EventListQuery{
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	bindings, err := s.svcCtx.Extensions.Installation.ListBindings(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	config := map[string]any{}
	secretRefs := map[string]string{}
	_ = json.Unmarshal([]byte(item.ConfigJSON), &config)
	_ = json.Unmarshal([]byte(item.SecretRefsJSON), &secretRefs)
	configSchema := s.resolveConfigSchema(ctx, item.ExtensionID, item.ReleaseVersion)
	return &ExtensionInstallationDetailResponse{
		Code:         200,
		Message:      "success",
		Installation: ptrInstallationItem(*item),
		ConfigSchema: configSchema,
		Config:       config,
		SecretRefs:   secretRefs,
		Bindings:     toBindingItems(bindings),
		Events:       toEventItems(events),
	}, nil
}

func (s *Service) UpdateConfig(ctx context.Context, id uint, req ExtensionConfigUpdateRequest, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权更新扩展配置"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	if err := s.validateInstallConfig(ctx, item.ExtensionID, item.ReleaseVersion, req.Config); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.UpdateConfig(ctx, id, req.Config, req.SecretRefs, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Code: 200, Message: "success", Status: "updated"}, nil
}

func (s *Service) ConfigSchema(ctx context.Context, id uint) (*ExtensionConfigSchemaResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展配置 schema"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	schema := s.resolveConfigSchema(ctx, item.ExtensionID, item.ReleaseVersion)
	return &ExtensionConfigSchemaResponse{
		Code:    200,
		Message: "success",
		Schema:  schema,
	}, nil
}

func (s *Service) Config(ctx context.Context, id uint) (*ExtensionConfigResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展配置"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	config := map[string]any{}
	secretRefs := map[string]string{}
	_ = json.Unmarshal([]byte(item.ConfigJSON), &config)
	_ = json.Unmarshal([]byte(item.SecretRefsJSON), &secretRefs)
	return &ExtensionConfigResponse{
		Code:       200,
		Message:    "success",
		Config:     config,
		SecretRefs: secretRefs,
	}, nil
}

func (s *Service) TestConnection(ctx context.Context, id uint, operator string) (*ExtensionTestConnectionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权测试扩展连接"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	_ = s.svcCtx.Extensions.Installation.RecordEvent(
		ctx,
		id,
		"test_connection",
		"info",
		"extension test-connection executed",
		operator,
		`{"status":"ok"}`,
	)
	return &ExtensionTestConnectionResponse{
		Code:    200,
		Message: "success",
		Status:  connectionStatusByInstallation(item),
	}, nil
}

func (s *Service) Capabilities(ctx context.Context, id uint) (*ExtensionCapabilitiesResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展能力"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	bindings, err := s.svcCtx.Extensions.Installation.ListBindings(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	caps, details := extractCapabilityDetailsFromBindings(bindings)
	// Fallback to manifest capabilities when bindings are empty.
	if len(caps) == 0 {
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err == nil {
			capSet := map[string]bool{}
			for _, cap := range caps {
				capSet[cap] = true
			}
			for _, key := range extractCapabilities(manifest) {
				if key == "" || capSet[key] {
					continue
				}
				capSet[key] = true
				caps = append(caps, key)
				details = append(details, ExtensionCapabilityDetail{
					Type:       "manifest",
					Key:        key,
					Capability: key,
					Operations: []string{},
					Source:     "manifest",
				})
			}
		}
	}
	return &ExtensionCapabilitiesResponse{
		Code:         200,
		Message:      "success",
		Capabilities: caps,
		Details:      details,
	}, nil
}

func (s *Service) Pages(ctx context.Context, id uint) (*ExtensionPagesResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展页面"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	bindings, err := s.svcCtx.Extensions.Installation.ListBindings(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	pages := extractPageDetailsFromBindings(bindings)
	if len(pages) == 0 {
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err == nil {
			if ui, ok := manifest["ui"].(map[string]any); ok {
				if rawPages, ok := ui["pages"].([]any); ok {
					for idx, raw := range rawPages {
						route := strings.TrimSpace(fmt.Sprint(raw))
						if route == "" || route == "<nil>" {
							continue
						}
						pages = append(pages, ExtensionPageItem{
							Type:   "manifest",
							Key:    route,
							Title:  route,
							Route:  route,
							Order:  idx + 1,
							Source: "manifest",
							Schema: map[string]any{},
						})
					}
				}
			}
		}
	}
	sort.SliceStable(pages, func(i, j int) bool {
		if pages[i].Order == pages[j].Order {
			return pages[i].Key < pages[j].Key
		}
		return pages[i].Order < pages[j].Order
	})
	return &ExtensionPagesResponse{
		Code:    200,
		Message: "success",
		Pages:   pages,
	}, nil
}

func extractCapabilityDetailsFromBindings(bindings []model.ExtensionRuntimeBinding) ([]string, []ExtensionCapabilityDetail) {
	capSet := map[string]bool{}
	caps := make([]string, 0)
	detailIndex := map[string]int{}
	details := make([]ExtensionCapabilityDetail, 0)
	addCap := func(capability string) {
		key := strings.TrimSpace(capability)
		if key == "" || capSet[key] {
			return
		}
		capSet[key] = true
		caps = append(caps, key)
	}
	appendOperation := func(detail *ExtensionCapabilityDetail, operations []string) {
		if detail == nil {
			return
		}
		seen := map[string]bool{}
		for _, op := range detail.Operations {
			seen[strings.TrimSpace(op)] = true
		}
		for _, op := range operations {
			key := strings.TrimSpace(op)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			detail.Operations = append(detail.Operations, key)
		}
	}
	for _, b := range bindings {
		raw := strings.TrimSpace(b.BindingType + ":" + b.BindingKey)
		if raw != "" && raw != ":" {
			addCap(raw)
		}
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		switch bt {
		case "provider", "openapi":
			spec := map[string]any{}
			if strings.TrimSpace(b.SpecJSON) != "" {
				_ = json.Unmarshal([]byte(b.SpecJSON), &spec)
			}
			parsed, ok := externalfunc.ParseProviderBinding(b.BindingKey, spec)
			if !ok {
				continue
			}
			capability := externalfunc.Capability(parsed.Provider)
			if capability == "" {
				continue
			}
			addCap(capability)
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], parsed.Operations)
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:       bt,
				Key:        parsed.Provider,
				Capability: capability,
				Provider:   parsed.Provider,
				Operations: append([]string{}, parsed.Operations...),
				Source:     "binding",
			})
		case "function":
			provider, method, ok := externalfunc.ParseFunctionID(strings.TrimSpace(b.BindingKey))
			if !ok {
				continue
			}
			provider = externalfunc.SanitizeKey(provider)
			method = externalfunc.SanitizeKey(method)
			if provider == "" || method == "" {
				continue
			}
			capability := externalfunc.Capability(provider)
			addCap(capability)
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], []string{method})
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:       bt,
				Key:        strings.TrimSpace(b.BindingKey),
				Capability: capability,
				Provider:   provider,
				Operations: []string{method},
				Source:     "binding",
			})
		}
	}
	return caps, details
}

func extractPageDetailsFromBindings(bindings []model.ExtensionRuntimeBinding) []ExtensionPageItem {
	items := make([]ExtensionPageItem, 0)
	seen := map[string]bool{}
	for _, b := range bindings {
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		if bt != "page" && bt != "ui" && bt != "navigation" {
			continue
		}
		spec := map[string]any{}
		if strings.TrimSpace(b.SpecJSON) != "" {
			_ = json.Unmarshal([]byte(b.SpecJSON), &spec)
		}
		key := strings.TrimSpace(b.BindingKey)
		if key == "" {
			key = strings.TrimSpace(fmt.Sprint(spec["id"]))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		title := strings.TrimSpace(fmt.Sprint(spec["title"]))
		route := strings.TrimSpace(fmt.Sprint(spec["route"]))
		icon := strings.TrimSpace(fmt.Sprint(spec["icon"]))
		group := strings.TrimSpace(fmt.Sprint(spec["group"]))
		order := 0
		if rawOrder, ok := spec["order"]; ok {
			switch v := rawOrder.(type) {
			case float64:
				order = int(v)
			case int:
				order = v
			case int64:
				order = int(v)
			}
		}
		if order <= 0 {
			order = len(items) + 1
		}
		if title == "" {
			title = key
		}
		if route == "" || route == "<nil>" {
			route = "/" + strings.ReplaceAll(key, ".", "/")
		}
		items = append(items, ExtensionPageItem{
			Type:   bt,
			Key:    key,
			Title:  title,
			Route:  route,
			Icon:   icon,
			Group:  group,
			Order:  order,
			Source: "binding",
			Schema: spec,
		})
	}
	return items
}

func (s *Service) HealthCheck(ctx context.Context, id uint, operator string) (*ExtensionHealthCheckResponse, error) {
	if err := s.requireWritePermission(ctx, "无权执行扩展健康检查"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	status := "unknown"
	if strings.EqualFold(item.Status, "uninstalled") || strings.EqualFold(item.DesiredState, "uninstalled") {
		status = "uninstalled"
	} else if item.Enabled {
		status = "healthy"
	} else {
		status = "disabled"
	}
	_ = s.svcCtx.Extensions.Installation.RecordEvent(
		ctx,
		id,
		"health_check",
		"info",
		"extension health-check executed",
		operator,
		fmt.Sprintf(`{"status":"%s"}`, status),
	)
	return &ExtensionHealthCheckResponse{
		Code:      200,
		Message:   "success",
		Status:    status,
		CheckedAt: time.Now().Unix(),
	}, nil
}

func connectionStatusByInstallation(item *model.ExtensionInstallation) string {
	if item == nil {
		return "unknown"
	}
	if strings.EqualFold(item.Status, "uninstalled") || strings.EqualFold(item.DesiredState, "uninstalled") {
		return "uninstalled"
	}
	if item.Enabled {
		return "ok"
	}
	return "disabled"
}

func (s *Service) Enable(ctx context.Context, id uint, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权启用扩展"); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Enable(ctx, id, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Code: 200, Message: "success", Status: "enabled"}, nil
}

func (s *Service) Disable(ctx context.Context, id uint, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权停用扩展"); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Disable(ctx, id, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Code: 200, Message: "success", Status: "disabled"}, nil
}

func (s *Service) Upgrade(ctx context.Context, id uint, version, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权升级扩展"); err != nil {
		return nil, err
	}
	targetVersion := strings.TrimSpace(version)
	if targetVersion == "" {
		return nil, errorx.NewBadRequest("release_version is required")
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	if strings.EqualFold(strings.TrimSpace(item.ReleaseVersion), targetVersion) {
		return nil, errorx.NewConflict("already on target release version")
	}
	exists, err := s.releaseVersionExists(ctx, item.ExtensionID, targetVersion)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errorx.NewBadRequest("target release version not found in catalog")
	}
	if err := s.validateDependencies(ctx, item.ExtensionID, targetVersion); err != nil {
		return nil, err
	}
	currentConfig := map[string]any{}
	if strings.TrimSpace(item.ConfigJSON) != "" {
		_ = json.Unmarshal([]byte(item.ConfigJSON), &currentConfig)
	}
	if err := s.validateInstallConfig(ctx, item.ExtensionID, targetVersion, currentConfig); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Upgrade(ctx, id, targetVersion, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Code: 200, Message: "success", Status: "upgraded"}, nil
}

func (s *Service) Reconcile(ctx context.Context, id uint) (*ExtensionReconcileResponse, error) {
	if err := s.requireWritePermission(ctx, "无权执行扩展重建"); err != nil {
		return nil, err
	}
	result, err := s.svcCtx.Extensions.Runtime.Reconcile(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionReconcileResponse{
		Code:    200,
		Message: "success",
		Status:  result.Status,
		Applied: result.Applied,
		Failed:  result.Failed,
	}, nil
}

func (s *Service) Uninstall(ctx context.Context, id uint, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权卸载扩展"); err != nil {
		return nil, err
	}
	if err := s.ensureNoActiveDependents(ctx, id); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Uninstall(ctx, id, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Code: 200, Message: "success", Status: "uninstalled"}, nil
}

func (s *Service) Events(ctx context.Context, id uint, req ExtensionEventListRequest) (*ExtensionEventListResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展事件"); err != nil {
		return nil, err
	}
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	events, total, err := s.svcCtx.Extensions.Installation.ListEvents(ctx, id, extensioninstallation.EventListQuery{
		Level:   req.Level,
		Keyword: strings.TrimSpace(req.Keyword),
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	items := toEventItems(events)
	return &ExtensionEventListResponse{Code: 200, Message: "success", Total: total, Items: items}, nil
}

func (s *Service) AgentSyncPayload(ctx context.Context, agentID string) (*ExtensionAgentSyncResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展同步数据"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, errorx.NewBadRequest("agent id is required")
	}
	payload, err := s.svcCtx.Extensions.Sync.BuildAgentPayload(ctx, agentID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionAgentSyncResponse{
		Code:    200,
		Message: "success",
		Payload: payload,
	}, nil
}

func firstManifest(items []model.ExtensionRelease) string {
	if len(items) == 0 {
		return "{}"
	}
	return items[0].ManifestJSON
}

func (s *Service) resolveConfigSchema(ctx context.Context, extensionID, releaseVersion string) map[string]any {
	if strings.TrimSpace(extensionID) == "" {
		return map[string]any{}
	}
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return map[string]any{}
	}
	raw := firstManifest(releases)
	for _, release := range releases {
		if release.Version == releaseVersion {
			raw = release.ManifestJSON
			break
		}
	}
	manifest := s.svcCtx.Extensions.Manifest.MustJSON(raw)
	if schema, ok := manifest["config_schema"].(map[string]any); ok {
		return schema
	}
	if schema, ok := manifest["configSchema"].(map[string]any); ok {
		return schema
	}
	return map[string]any{}
}

func (s *Service) validateInstallConfig(ctx context.Context, extensionID, releaseVersion string, config map[string]any) error {
	schema := s.resolveConfigSchema(ctx, extensionID, releaseVersion)
	if len(schema) == 0 {
		return nil
	}
	return validateConfigAgainstSchema(config, schema)
}

func validateConfigAgainstSchema(config map[string]any, schema map[string]any) error {
	if config == nil {
		config = map[string]any{}
	}
	properties, _ := schema["properties"].(map[string]any)
	requiredKeys, _ := schema["required"].([]any)

	for _, rawKey := range requiredKeys {
		key := strings.TrimSpace(fmt.Sprint(rawKey))
		if key == "" {
			continue
		}
		if _, ok := config[key]; !ok {
			return errorx.NewBadRequest("invalid config: missing required field " + key)
		}
	}

	for key, value := range config {
		rawRule, ok := properties[key]
		if !ok {
			continue
		}
		rule, _ := rawRule.(map[string]any)
		if rule == nil {
			continue
		}
		if err := validateConfigField(key, value, rule); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigField(key string, value any, rule map[string]any) error {
	if enums, ok := rule["enum"].([]any); ok && len(enums) > 0 {
		matched := false
		for _, option := range enums {
			if fmt.Sprint(option) == fmt.Sprint(value) {
				matched = true
				break
			}
		}
		if !matched {
			return errorx.NewBadRequest("invalid config: field " + key + " not in enum")
		}
	}

	typeName := strings.ToLower(strings.TrimSpace(fmt.Sprint(rule["type"])))
	if typeName == "" {
		return nil
	}

	switch typeName {
	case "string":
		if _, ok := value.(string); !ok {
			return errorx.NewBadRequest("invalid config: field " + key + " must be string")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errorx.NewBadRequest("invalid config: field " + key + " must be boolean")
		}
	case "number":
		if !isJSONNumberType(value) {
			return errorx.NewBadRequest("invalid config: field " + key + " must be number")
		}
	case "integer":
		if !isJSONIntegerType(value) {
			return errorx.NewBadRequest("invalid config: field " + key + " must be integer")
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return errorx.NewBadRequest("invalid config: field " + key + " must be object")
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return errorx.NewBadRequest("invalid config: field " + key + " must be array")
		}
	}
	return nil
}

func isJSONNumberType(v any) bool {
	switch v.(type) {
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}

func isJSONIntegerType(v any) bool {
	switch n := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return n == float64(int64(n))
	case float32:
		return n == float32(int64(n))
	default:
		return false
	}
}

func toInstallationItem(item model.ExtensionInstallation) ExtensionInstallationItem {
	return ExtensionInstallationItem{
		ID:              item.ID,
		InstallationKey: item.InstallationKey,
		ExtensionID:     item.ExtensionID,
		DisplayName:     item.ExtensionID,
		ReleaseVersion:  item.ReleaseVersion,
		ScopeType:       item.ScopeType,
		ScopeID:         item.ScopeID,
		TargetType:      item.TargetType,
		TargetID:        item.TargetID,
		Status:          item.Status,
		DesiredState:    item.DesiredState,
		Enabled:         item.Enabled,
		HealthStatus:    "unknown",
		LastError:       item.LastError,
		UpdatedAt:       item.UpdatedAt.Unix(),
	}
}

func ptrInstallationItem(item model.ExtensionInstallation) *ExtensionInstallationItem {
	out := toInstallationItem(item)
	return &out
}

func toEventItems(events []model.ExtensionEvent) []ExtensionEventItem {
	items := make([]ExtensionEventItem, 0, len(events))
	for _, event := range events {
		items = append(items, ExtensionEventItem{
			EventType: event.EventType,
			Level:     event.Level,
			Message:   event.Message,
			Payload:   event.PayloadJSON,
			CreatedBy: event.CreatedBy,
			CreatedAt: event.CreatedAt.Unix(),
		})
	}
	return items
}

func toBindingItems(bindings []model.ExtensionRuntimeBinding) []ExtensionBindingItem {
	items := make([]ExtensionBindingItem, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, ExtensionBindingItem{
			BindingType: binding.BindingType,
			BindingKey:  binding.BindingKey,
			TargetRef:   binding.TargetRef,
			Status:      binding.Status,
			LastError:   binding.LastError,
		})
	}
	return items
}

func catalogListQuery(req ExtensionCatalogListRequest) extensioncatalog.ListQuery {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return extensioncatalog.ListQuery{
		Keyword: req.Keyword,
		Kind:    req.Kind,
		Status:  req.Status,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	}
}

func installationListQuery(req ExtensionInstallationListRequest) extensioninstallation.ListQuery {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return extensioninstallation.ListQuery{
		ExtensionID: req.ExtensionID,
		ScopeType:   req.ScopeType,
		ScopeID:     req.ScopeID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Status:      req.Status,
		Enabled:     req.Enabled,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}
}

func mapServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errorx.NewNotFound("resource not found")
	}
	if errors.Is(err, gorm.ErrInvalidDB) {
		return errorx.NewInternalError("extension service not initialized")
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errorx.NewConflict("resource already exists")
	}
	return err
}

func (s *Service) requireReadPermission(ctx context.Context, message string) error {
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, message,
		"admin:all",
		"extension:read",
		"extensions:read",
		"extension:manage",
		"extensions:manage",
	)
	return err
}

func (s *Service) requireWritePermission(ctx context.Context, message string) error {
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, message,
		"admin:all",
		"extension:write",
		"extensions:write",
		"extension:manage",
		"extensions:manage",
	)
	return err
}

func (s *Service) ResolveInstallationID(ctx context.Context, identifier string) (uint, error) {
	if id, err := strconv.ParseUint(strings.TrimSpace(identifier), 10, 64); err == nil {
		return uint(id), nil
	}
	item, err := s.findActiveInstallationByExtension(ctx, identifier)
	if err != nil {
		return 0, err
	}
	if item == nil {
		return 0, errorx.NewNotFound("installation not found for extension id")
	}
	return item.ID, nil
}

func (s *Service) findInstallationConflict(ctx context.Context, req ExtensionInstallRequest) (bool, *model.ExtensionInstallation, error) {
	db := s.svcCtx.DB
	if db == nil {
		return false, nil, errorx.NewInternalError("database is not initialized")
	}
	var item model.ExtensionInstallation
	err := db.WithContext(ctx).
		Model(&model.ExtensionInstallation{}).
		Where("extension_id = ? AND scope_type = ? AND scope_id = ? AND target_type = ? AND target_id = ?",
			req.ExtensionID, req.ScopeType, req.ScopeID, req.TargetType, req.TargetID).
		Where("LOWER(status) <> ? AND LOWER(desired_state) <> ?", "uninstalled", "uninstalled").
		Order("id DESC").
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		return false, nil, mapServiceError(err)
	}
	return true, &item, nil
}

func (s *Service) releaseVersionExists(ctx context.Context, extensionID, releaseVersion string) (bool, error) {
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return false, mapServiceError(err)
	}
	target := strings.TrimSpace(releaseVersion)
	for _, release := range releases {
		if strings.EqualFold(strings.TrimSpace(release.Version), target) {
			return true, nil
		}
	}
	return false, nil
}

type extensionDependency struct {
	ExtensionID string
	Version     string
}

func (s *Service) validateDependencies(ctx context.Context, extensionID, releaseVersion string) error {
	manifest, err := s.resolveManifestForRelease(ctx, extensionID, releaseVersion)
	if err != nil {
		return err
	}
	deps := parseDependencies(manifest)
	path := map[string]bool{normalizeExtensionID(extensionID): true}
	visited := map[string]bool{}
	for _, dep := range deps {
		if strings.TrimSpace(dep.ExtensionID) == "" {
			continue
		}
		if err := s.validateDependencyNode(ctx, dep, path, visited); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateDependencyNode(
	ctx context.Context,
	dep extensionDependency,
	path map[string]bool,
	visited map[string]bool,
) error {
	id := normalizeExtensionID(dep.ExtensionID)
	if id == "" {
		return nil
	}
	if path[id] {
		return errorx.NewBadRequestWithDetails(
			"dependency cycle detected at extension: "+dep.ExtensionID,
			map[string]any{
				"code":       "dependency_cycle",
				"dependency": dep.ExtensionID,
			},
		)
	}

	installed, err := s.findActiveInstallationByExtension(ctx, dep.ExtensionID)
	if err != nil {
		return err
	}
	if installed == nil {
		return errorx.NewBadRequestWithDetails(
			"missing dependency extension: "+dep.ExtensionID,
			map[string]any{
				"code":       "dependency_missing",
				"dependency": dep.ExtensionID,
			},
		)
	}
	if strings.TrimSpace(dep.Version) != "" &&
		!matchVersionConstraint(installed.ReleaseVersion, dep.Version) {
		return errorx.NewBadRequestWithDetails(
			"dependency version mismatch: "+dep.ExtensionID+
				", required: "+dep.Version+", current: "+installed.ReleaseVersion,
			map[string]any{
				"code":             "dependency_version_mismatch",
				"dependency":       dep.ExtensionID,
				"required_version": dep.Version,
				"current_version":  installed.ReleaseVersion,
			},
		)
	}

	visitKey := id + "@" + strings.ToLower(strings.TrimSpace(installed.ReleaseVersion))
	if visited[visitKey] {
		return nil
	}
	visited[visitKey] = true

	path[id] = true
	defer delete(path, id)

	manifest, err := s.resolveManifestForRelease(ctx, installed.ExtensionID, installed.ReleaseVersion)
	if err != nil {
		return err
	}
	children := parseDependencies(manifest)
	for _, child := range children {
		if strings.TrimSpace(child.ExtensionID) == "" {
			continue
		}
		if err := s.validateDependencyNode(ctx, child, path, visited); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) resolveManifestForRelease(ctx context.Context, extensionID, releaseVersion string) (map[string]any, error) {
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	target := strings.TrimSpace(releaseVersion)
	raw := ""
	for _, release := range releases {
		if strings.EqualFold(strings.TrimSpace(release.Version), target) {
			raw = release.ManifestJSON
			break
		}
	}
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	return s.svcCtx.Extensions.Manifest.MustJSON(raw), nil
}

func (s *Service) findActiveInstallationByExtension(ctx context.Context, extensionID string) (*model.ExtensionInstallation, error) {
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: extensionID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, mapServiceError(err)
	}
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") &&
			!strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			return &item, nil
		}
	}
	return nil, nil
}

func (s *Service) activeInstalledExtensionSet(ctx context.Context) (map[string]bool, error) {
	result := map[string]bool{}
	page := 1
	pageSize := 200
	for {
		items, total, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
			Limit:  pageSize,
			Offset: (page - 1) * pageSize,
		})
		if err != nil {
			return nil, mapServiceError(err)
		}
		for _, item := range items {
			if !isActiveInstallation(item) {
				continue
			}
			id := normalizeExtensionID(item.ExtensionID)
			if id != "" {
				result[id] = true
			}
		}
		if int64(page*pageSize) >= total || len(items) == 0 {
			break
		}
		page++
	}
	return result, nil
}

func parseDependencies(manifest map[string]any) []extensionDependency {
	if manifest == nil {
		return nil
	}
	rawDeps, ok := manifest["dependencies"]
	if !ok {
		return nil
	}
	list, ok := rawDeps.([]any)
	if !ok {
		return nil
	}
	out := make([]extensionDependency, 0, len(list))
	for _, raw := range list {
		switch v := raw.(type) {
		case string:
			id := strings.TrimSpace(v)
			if id != "" {
				out = append(out, extensionDependency{ExtensionID: id})
			}
		case map[string]any:
			id := mapString(v, "id")
			if id == "" {
				id = mapString(v, "extension_id")
			}
			if id == "" {
				continue
			}
			version := mapString(v, "version")
			if version == "" {
				version = mapString(v, "required_version")
			}
			out = append(out, extensionDependency{ExtensionID: id, Version: version})
		}
	}
	return out
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(raw))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func normalizeExtensionID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func extractCapabilities(manifest map[string]any) []string {
	if manifest == nil {
		return []string{}
	}
	raw, ok := manifest["capabilities"]
	if !ok {
		return []string{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		switch v := item.(type) {
		case string:
			key := strings.TrimSpace(v)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		case map[string]any:
			key := mapString(v, "id")
			if key == "" {
				key = mapString(v, "name")
			}
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func extractTags(manifest map[string]any) []string {
	if manifest == nil {
		return []string{}
	}
	raw, ok := manifest["tags"]
	if !ok {
		return []string{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		tag := strings.TrimSpace(fmt.Sprint(item))
		if tag == "" || tag == "<nil>" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func extractDefaultInstall(manifest map[string]any) bool {
	if manifest == nil {
		return false
	}
	raw, ok := manifest["default_install"]
	if !ok {
		raw = manifest["defaultInstall"]
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func (s *Service) resolveCatalogMetadata(ctx context.Context, extensionID, latestVersion string) (bool, []string) {
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil || len(releases) == 0 {
		return false, []string{}
	}
	raw := firstManifest(releases)
	for _, r := range releases {
		if strings.EqualFold(strings.TrimSpace(r.Version), strings.TrimSpace(latestVersion)) {
			raw = r.ManifestJSON
			break
		}
	}
	manifest := s.svcCtx.Extensions.Manifest.MustJSON(raw)
	return extractDefaultInstall(manifest), extractTags(manifest)
}

func isActiveInstallation(item model.ExtensionInstallation) bool {
	return !strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") &&
		!strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled")
}

func dependencyTargetsExtension(dep extensionDependency, extensionID, releaseVersion string) bool {
	if normalizeExtensionID(dep.ExtensionID) != normalizeExtensionID(extensionID) {
		return false
	}
	if strings.TrimSpace(dep.Version) == "" {
		return true
	}
	return matchVersionConstraint(releaseVersion, dep.Version)
}

func (s *Service) ensureNoActiveDependents(ctx context.Context, installationID uint) error {
	current, err := s.svcCtx.Extensions.Installation.Get(ctx, installationID)
	if err != nil {
		return mapServiceError(err)
	}
	all, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		return mapServiceError(err)
	}
	blockers := make([]string, 0)
	for _, item := range all {
		if item.ID == current.ID || !isActiveInstallation(item) {
			continue
		}
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err != nil {
			return err
		}
		deps := parseDependencies(manifest)
		for _, dep := range deps {
			if dependencyTargetsExtension(dep, current.ExtensionID, current.ReleaseVersion) {
				blockers = append(blockers, formatDependentRef(item))
				break
			}
		}
	}
	if len(blockers) > 0 {
		return errorx.NewConflictWithDetails(
			"extension is required by installed extensions: "+strings.Join(blockers, ", "),
			map[string]any{
				"code":     "dependency_blocked",
				"blockers": blockers,
			},
		)
	}
	return nil
}

func formatDependentRef(item model.ExtensionInstallation) string {
	id := strings.TrimSpace(item.ExtensionID)
	ver := strings.TrimSpace(item.ReleaseVersion)
	if id == "" {
		id = "unknown"
	}
	if ver == "" {
		return id
	}
	return id + "@" + ver
}

type semVersion struct {
	major int
	minor int
	patch int
	parts int
}

func matchVersionConstraint(current, constraint string) bool {
	cur, ok := parseSemVersion(current)
	if !ok {
		return false
	}
	c := strings.TrimSpace(constraint)
	if c == "" {
		return true
	}
	clauses := strings.Split(c, ",")
	for _, rawClause := range clauses {
		clause := strings.TrimSpace(rawClause)
		if clause == "" {
			continue
		}
		if !matchSingleClause(cur, clause) {
			return false
		}
	}
	return true
}

func matchSingleClause(current semVersion, clause string) bool {
	op := ""
	rest := clause
	switch {
	case strings.HasPrefix(clause, ">="), strings.HasPrefix(clause, "<="):
		op, rest = clause[:2], clause[2:]
	case strings.HasPrefix(clause, ">"), strings.HasPrefix(clause, "<"),
		strings.HasPrefix(clause, "="), strings.HasPrefix(clause, "^"), strings.HasPrefix(clause, "~"):
		op, rest = clause[:1], clause[1:]
	default:
		op = "="
	}
	target, ok := parseSemVersion(rest)
	if !ok {
		return false
	}
	cmp := compareSemVersion(current, target)

	switch op {
	case "=":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "^":
		return matchCaretConstraint(current, target)
	case "~":
		return matchTildeConstraint(current, target)
	default:
		return false
	}
}

func matchCaretConstraint(current, base semVersion) bool {
	if compareSemVersion(current, base) < 0 {
		return false
	}
	upper := semVersion{}
	if base.major > 0 {
		upper = semVersion{major: base.major + 1}
	} else if base.minor > 0 {
		upper = semVersion{major: 0, minor: base.minor + 1}
	} else {
		upper = semVersion{major: 0, minor: 0, patch: base.patch + 1}
	}
	return compareSemVersion(current, upper) < 0
}

func matchTildeConstraint(current, base semVersion) bool {
	if compareSemVersion(current, base) < 0 {
		return false
	}
	upper := semVersion{}
	if base.parts <= 1 {
		upper = semVersion{major: base.major + 1}
	} else {
		upper = semVersion{major: base.major, minor: base.minor + 1}
	}
	return compareSemVersion(current, upper) < 0
}

func parseSemVersion(raw string) (semVersion, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if s == "" {
		return semVersion{}, false
	}
	if idx := strings.IndexAny(s, "+-"); idx >= 0 {
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semVersion{}, false
	}
	out := semVersion{parts: len(parts)}
	parsePart := func(i int, dst *int) bool {
		if i >= len(parts) {
			*dst = 0
			return true
		}
		p := strings.TrimSpace(parts[i])
		if p == "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return false
		}
		*dst = n
		return true
	}
	if !parsePart(0, &out.major) || !parsePart(1, &out.minor) || !parsePart(2, &out.patch) {
		return semVersion{}, false
	}
	return out, true
}

func compareSemVersion(a, b semVersion) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return 0
}
