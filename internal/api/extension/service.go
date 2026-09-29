package extension

import (
	"bytes"
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
	return &ExtensionCatalogListResponse{Total: total, Items: respItems}, nil
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
		Total:    int64(len(items)),
		Releases: items,
	}, nil
}

// ---- catalog 写路径（#46 批次 3）：登记 / 更新（含上下架）/ 移除 / 发布版本 ----

// catalogStatuses：上下架语义闭集（delisted=下架，列表与安装入口按 status 过滤）。
var catalogStatuses = map[string]bool{"active": true, "delisted": true}

// catalogReleaseChannels：发布渠道闭集。
var catalogReleaseChannels = map[string]bool{"stable": true, "beta": true, "alpha": true}

// validateCatalogExtensionID：extension id 形态约束（小写字母/数字开头，允许 . _ -，
// 与 official.<domain> 命名规则兼容，禁空白防路由参数歧义）。
func validateCatalogExtensionID(id string) error {
	if id == "" {
		return errorx.NewBadRequest("extensionId is required")
	}
	if len(id) > 128 {
		return errorx.NewBadRequest("extensionId must be at most 128 characters")
	}
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
			if i == 0 {
				return errorx.NewBadRequest("extensionId must start with a letter or digit")
			}
		default:
			return errorx.NewBadRequest("extensionId only allows lowercase letters, digits, '.', '_', '-'")
		}
	}
	return nil
}

// catalogItemFromModel：写路径返回体组装（复用读侧 item 形态，不含 installed 派生）。
func catalogItemFromModel(item *model.ExtensionCatalog) ExtensionCatalogItem {
	return ExtensionCatalogItem{
		ID:            item.ExtensionID,
		Name:          item.Name,
		DisplayName:   item.DisplayName,
		Vendor:        item.Vendor,
		Kind:          item.Kind,
		Summary:       item.Summary,
		IconURL:       item.IconURL,
		Status:        item.Status,
		LatestVersion: item.LatestVersion,
	}
}

func (s *Service) CatalogCreate(ctx context.Context, req ExtensionCatalogCreateRequest, operator string) (*ExtensionCatalogMutateResponse, error) {
	if err := s.requireWritePermission(ctx, "无权登记扩展"); err != nil {
		return nil, err
	}
	extID := strings.TrimSpace(req.ExtensionID)
	if err := validateCatalogExtensionID(extID); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	if !catalogStatuses[status] {
		return nil, errorx.NewBadRequest("invalid status: only active/delisted allowed")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "community"
	}
	if _, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, extID); err == nil {
		return nil, errorx.NewConflict("extension already exists in catalog")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, mapServiceError(err)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = extID
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = name
	}
	vendor := strings.TrimSpace(req.Vendor)
	if vendor == "" {
		vendor = "external"
	}
	record := &model.ExtensionCatalog{
		ExtensionID:   extID,
		Name:          name,
		DisplayName:   displayName,
		Vendor:        vendor,
		Kind:          kind,
		Summary:       strings.TrimSpace(req.Summary),
		IconURL:       strings.TrimSpace(req.IconURL),
		HomepageURL:   strings.TrimSpace(req.HomepageURL),
		Status:        status,
		LatestVersion: strings.TrimSpace(req.LatestVersion),
	}
	if err := s.svcCtx.Extensions.Catalog.Create(ctx, record); err != nil {
		return nil, mapServiceError(err)
	}
	_ = operator // 审计经 HTTP 层统一审计链；catalog 行无 createdBy 列
	return &ExtensionCatalogMutateResponse{Item: catalogItemFromModel(record)}, nil
}

func (s *Service) CatalogUpdate(ctx context.Context, extensionID string, req ExtensionCatalogUpdateRequest) (*ExtensionCatalogMutateResponse, error) {
	if err := s.requireWritePermission(ctx, "无权修改扩展登记"); err != nil {
		return nil, err
	}
	existing, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	updates := map[string]any{}
	if v := strings.TrimSpace(req.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(req.DisplayName); v != "" {
		updates["display_name"] = v
	}
	if v := strings.TrimSpace(req.Vendor); v != "" {
		updates["vendor"] = v
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		updates["kind"] = v
	}
	updates["summary"] = strings.TrimSpace(req.Summary)
	updates["icon_url"] = strings.TrimSpace(req.IconURL)
	updates["homepage_url"] = strings.TrimSpace(req.HomepageURL)
	if v := strings.TrimSpace(req.Status); v != "" {
		if !catalogStatuses[v] {
			return nil, errorx.NewBadRequest("invalid status: only active/delisted allowed")
		}
		updates["status"] = v
	}
	if len(updates) > 0 {
		if err := s.svcCtx.Extensions.Catalog.UpdateFields(ctx, existing.ExtensionID, updates); err != nil {
			return nil, mapServiceError(err)
		}
	}
	updated, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, existing.ExtensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionCatalogMutateResponse{Item: catalogItemFromModel(updated)}, nil
}

func (s *Service) CatalogDelete(ctx context.Context, extensionID string) error {
	if err := s.requireWritePermission(ctx, "无权移除扩展登记"); err != nil {
		return err
	}
	existing, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return mapServiceError(err)
	}
	// 有活跃安装实例时拒绝移除（先卸载再移除，避免悬挂引用）
	if err := s.rejectActiveInstallations(ctx, existing.ExtensionID); err != nil {
		return err
	}
	if err := s.svcCtx.Extensions.Catalog.RemoveReleases(ctx, existing.ExtensionID); err != nil {
		return mapServiceError(err)
	}
	return mapServiceError(s.svcCtx.Extensions.Catalog.Remove(ctx, existing.ExtensionID))
}

// rejectActiveInstallations：存在未卸载安装实例时拒绝 catalog 移除/下架外的破坏性操作。
func (s *Service) rejectActiveInstallations(ctx context.Context, extensionID string) error {
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: extensionID,
		Limit:       1,
	})
	if err != nil {
		return mapServiceError(err)
	}
	for _, item := range items {
		if !strings.EqualFold(item.Status, "uninstalled") && !strings.EqualFold(item.DesiredState, "uninstalled") {
			return errorx.NewConflictWithDetails(
				"extension has active installations; uninstall first",
				map[string]any{"installationId": item.ID},
			)
		}
	}
	return nil
}

func (s *Service) CatalogReleasePublish(ctx context.Context, extensionID string, req ExtensionReleasePublishRequest, operator string) (*ExtensionReleasePublishResponse, error) {
	if err := s.requireWritePermission(ctx, "无权发布扩展版本"); err != nil {
		return nil, err
	}
	existing, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil {
		return nil, mapServiceError(err)
	}
	version := strings.TrimSpace(req.Version)
	parsed, ok := parseSemVersion(version)
	if !ok {
		return nil, errorx.NewBadRequest("invalid version: expected semver like 1.2.3")
	}
	channel := strings.TrimSpace(req.ReleaseChannel)
	if channel == "" {
		channel = "stable"
	}
	if !catalogReleaseChannels[channel] {
		return nil, errorx.NewBadRequest("invalid releaseChannel: only stable/beta/alpha allowed")
	}
	if req.Manifest == nil {
		return nil, errorx.NewBadRequest("manifest is required")
	}
	if _, dup := s.svcCtx.Extensions.Catalog.ReleaseByVersion(ctx, existing.ExtensionID, version); dup == nil {
		return nil, errorx.NewConflict("release version already exists")
	} else if !errors.Is(dup, gorm.ErrRecordNotFound) {
		return nil, mapServiceError(dup)
	}
	manifestJSON, err := json.Marshal(req.Manifest)
	if err != nil {
		return nil, errorx.NewBadRequest("invalid manifest: must be a JSON object")
	}
	release := &model.ExtensionRelease{
		ExtensionID:     existing.ExtensionID,
		Version:         version,
		ReleaseChannel:  channel,
		ManifestJSON:    model.JSON(manifestJSON),
		PackageRef:      strings.TrimSpace(req.PackageRef),
		Checksum:        strings.TrimSpace(req.Checksum),
		MinCoreVersion:  strings.TrimSpace(req.MinCoreVersion),
		Changelog:       strings.TrimSpace(req.Changelog),
		PublishedAtUnix: time.Now().Unix(),
	}
	if err := s.svcCtx.Extensions.Catalog.PublishRelease(ctx, release); err != nil {
		return nil, mapServiceError(err)
	}
	// latestVersion 语义：仅当新版本 semver 更高时回滚 catalog 指针（降渠道补发不回退 latest）
	if shouldBumpLatestVersion(existing.LatestVersion, parsed) {
		if err := s.svcCtx.Extensions.Catalog.UpdateFields(ctx, existing.ExtensionID, map[string]any{"latest_version": version}); err != nil {
			return nil, mapServiceError(err)
		}
	}
	_ = operator
	return &ExtensionReleasePublishResponse{Release: ExtensionReleaseItem{
		Version:        release.Version,
		ReleaseChannel: release.ReleaseChannel,
		MinCoreVersion: release.MinCoreVersion,
		PublishedAt:    release.PublishedAtUnix,
		Changelog:      release.Changelog,
	}}, nil
}

// shouldBumpLatestVersion：catalog 无 latest 或新版本 semver 严格更高时回填。
func shouldBumpLatestVersion(current string, candidate semVersion) bool {
	curRaw := strings.TrimSpace(current)
	if curRaw == "" {
		return true
	}
	cur, ok := parseSemVersion(curRaw)
	if !ok {
		return true
	}
	if candidate.major != cur.major {
		return candidate.major > cur.major
	}
	if candidate.minor != cur.minor {
		return candidate.minor > cur.minor
	}
	return candidate.patch > cur.patch
}

func (s *Service) Install(ctx context.Context, req ExtensionInstallRequest, operator string) (*ExtensionInstallResponse, error) {
	if err := s.requireWritePermission(ctx, "无权安装扩展"); err != nil {
		return nil, err
	}
	if err := normalizeAndValidateExtensionScope(&req.ScopeType, &req.ScopeID); err != nil {
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
	return &ExtensionInstallResponse{InstallationID: item.ID, Status: item.Status}, nil
}

func (s *Service) InstallationList(ctx context.Context, req ExtensionInstallationListRequest) (*ExtensionInstallationListResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展安装实例"); err != nil {
		return nil, err
	}
	if err := normalizeAndValidateExtensionScope(&req.ScopeType, &req.ScopeID); err != nil {
		return nil, err
	}
	items, total, err := s.svcCtx.Extensions.Installation.List(ctx, installationListQuery(req))
	if err != nil {
		return nil, mapServiceError(err)
	}
	displayNames := s.catalogDisplayNames(ctx, items)
	respItems := make([]ExtensionInstallationItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toInstallationItem(item, displayNames[normalizeExtensionID(item.ExtensionID)]))
	}
	return &ExtensionInstallationListResponse{Total: total, Items: respItems}, nil
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
	_ = json.Unmarshal(item.ConfigJSON, &config)
	_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	configSchema := s.resolveConfigSchema(ctx, item.ExtensionID, item.ReleaseVersion)
	displayNames := s.catalogDisplayNames(ctx, []model.ExtensionInstallation{*item})
	return &ExtensionInstallationDetailResponse{
		Installation: ptrInstallationItem(*item, displayNames[normalizeExtensionID(item.ExtensionID)]),
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
	return &ExtensionActionResponse{Status: "updated"}, nil
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
		Schema: schema,
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
	_ = json.Unmarshal(item.ConfigJSON, &config)
	_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	return &ExtensionConfigResponse{
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
		Status: connectionStatusByInstallation(item),
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
	// 设计债清理：本块仅在 len(caps)==0 时进入，原「遍历 caps 预填 capSet」的
	// 循环体恒不可执行；extractCapabilities 产物恒非空串且内部已去重，原先的
	// 空 key/重复 key continue 防御同样恒假——两处死分支已删除。
	if len(caps) == 0 {
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err == nil {
			for _, key := range extractCapabilities(manifest) {
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
		Pages: pages,
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
		// 设计债清理：调用方恒传 &details[idx]，detail 恒非 nil，
		// 原 nil 防御分支不可达已删除。
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
	appendPermissions := func(detail *ExtensionCapabilityDetail, permissions map[string]string) {
		if len(permissions) == 0 {
			return
		}
		if detail.Permissions == nil {
			detail.Permissions = map[string]string{}
		}
		// 设计债清理：permissions 来自 parseStringMapAny，其产出已过滤空 key 与
		// 空 value，原空串 continue 分支不可达已删除。
		for op, perm := range permissions {
			detail.Permissions[op] = perm
		}
	}
	appendConfigKeys := func(detail *ExtensionCapabilityDetail, keys []string) {
		if detail == nil || len(keys) == 0 {
			return
		}
		seen := map[string]bool{}
		for _, key := range detail.ConfigKeys {
			seen[strings.TrimSpace(key)] = true
		}
		for _, key := range keys {
			k := strings.TrimSpace(key)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			detail.ConfigKeys = append(detail.ConfigKeys, k)
		}
	}
	for _, b := range bindings {
		raw := strings.TrimSpace(b.BindingType + ":" + b.BindingKey)
		if raw != "" && raw != ":" {
			addCap(raw)
		}
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		switch bt {
		case "capability":
			capability := strings.TrimSpace(b.BindingKey)
			if capability == "" {
				continue
			}
			addCap(capability)
			spec := map[string]any{}
			if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
				_ = json.Unmarshal(b.SpecJSON, &spec)
			}
			operations := parseStringSliceAny(spec["operations"])
			permissions := parseStringMapAny(spec["permissions"])
			configKeys := parseStringSliceAny(spec["config_keys"])
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], operations)
				appendPermissions(&details[idx], permissions)
				appendConfigKeys(&details[idx], configKeys)
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:        bt,
				Key:         capability,
				Capability:  capability,
				Operations:  operations,
				Permissions: permissions,
				ConfigKeys:  configKeys,
				Source:      "binding",
			})
		case "provider", "openapi":
			spec := map[string]any{}
			if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
				_ = json.Unmarshal(b.SpecJSON, &spec)
			}
			parsed, ok := externalfunc.ParseProviderBinding(b.BindingKey, spec)
			if !ok {
				continue
			}
			// 设计债清理：ok 时 parsed.Provider 是 SanitizeKey 的非空产物
			// （仅 [a-z0-9_.-]），Capability 对其再 SanitizeKey 幂等非空，
			// 原 capability=="" 分支不可达已删除。
			capability := externalfunc.Capability(parsed.Provider)
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
		if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
			_ = json.Unmarshal(b.SpecJSON, &spec)
		}
		key := strings.TrimSpace(b.BindingKey)
		if key == "" {
			key = strings.TrimSpace(fmt.Sprint(spec["id"]))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		specString := func(field string) string {
			value := strings.TrimSpace(fmt.Sprint(spec[field]))
			if value == "<nil>" {
				return ""
			}
			return value
		}
		title := specString("title")
		route := specString("route")
		icon := specString("icon")
		group := specString("group")
		requiredPermission := specString("required_permission")
		order := 0
		if rawOrder, ok := spec["order"]; ok {
			// 设计债清理：spec 为 json.Unmarshal 产物（map[string]any），
			// JSON 数字的动态类型恒为 float64，原 int/int64 分支不可达已删除。
			if v, isNum := rawOrder.(float64); isNum {
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
			Type:               bt,
			Key:                key,
			Title:              title,
			Route:              route,
			Icon:               icon,
			Group:              group,
			Order:              order,
			RequiredPermission: requiredPermission,
			Source:             "binding",
			Schema:             spec,
		})
	}
	return items
}

func parseStringSliceAny(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s == "" || s == "<nil>" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func parseStringMapAny(raw any) map[string]string {
	m, ok := raw.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(fmt.Sprint(v))
		if key == "" || val == "" || val == "<nil>" {
			continue
		}
		out[key] = val
	}
	return out
}

func (s *Service) HealthCheck(ctx context.Context, id uint, operator string) (*ExtensionHealthCheckResponse, error) {
	if err := s.requireWritePermission(ctx, "无权执行扩展健康检查"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	status := deriveExtensionHealthStatus(item)
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
	return &ExtensionActionResponse{Status: "enabled"}, nil
}

func (s *Service) Disable(ctx context.Context, id uint, operator string) (*ExtensionActionResponse, error) {
	if err := s.requireWritePermission(ctx, "无权停用扩展"); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Disable(ctx, id, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Status: "disabled"}, nil
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
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &currentConfig)
	}
	if err := s.validateInstallConfig(ctx, item.ExtensionID, targetVersion, currentConfig); err != nil {
		return nil, err
	}
	if err := s.svcCtx.Extensions.Installation.Upgrade(ctx, id, targetVersion, operator); err != nil {
		return nil, mapServiceError(err)
	}
	return &ExtensionActionResponse{Status: "upgraded"}, nil
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
	return &ExtensionActionResponse{Status: "uninstalled"}, nil
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
	return &ExtensionEventListResponse{Total: total, Items: items}, nil
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
		Payload: payload,
	}, nil
}

func firstManifest(items []model.ExtensionRelease) string {
	if len(items) == 0 {
		return "{}"
	}
	return string(items[0].ManifestJSON)
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
			raw = string(release.ManifestJSON)
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

func toInstallationItem(item model.ExtensionInstallation, displayName string) ExtensionInstallationItem {
	if strings.TrimSpace(displayName) == "" {
		displayName = item.ExtensionID
	}
	return ExtensionInstallationItem{
		ID:              item.ID,
		InstallationKey: item.InstallationKey,
		ExtensionID:     item.ExtensionID,
		DisplayName:     displayName,
		ReleaseVersion:  item.ReleaseVersion,
		ScopeType:       item.ScopeType,
		ScopeID:         item.ScopeID,
		TargetType:      item.TargetType,
		TargetID:        item.TargetID,
		Status:          item.Status,
		DesiredState:    item.DesiredState,
		Enabled:         item.Enabled,
		HealthStatus:    deriveExtensionHealthStatus(&item),
		LastError:       item.LastError,
		UpdatedAt:       item.UpdatedAt.Unix(),
	}
}

func ptrInstallationItem(item model.ExtensionInstallation, displayName string) *ExtensionInstallationItem {
	out := toInstallationItem(item, displayName)
	return &out
}

// deriveExtensionHealthStatus 与 HealthCheck 同口径（无健康探测落库，按 status/enabled 推导）：
// uninstalled（status 或 desired_state）→ uninstalled；enabled → healthy；否则 disabled。
func deriveExtensionHealthStatus(item *model.ExtensionInstallation) string {
	if item == nil {
		return "unknown"
	}
	if strings.EqualFold(item.Status, "uninstalled") || strings.EqualFold(item.DesiredState, "uninstalled") {
		return "uninstalled"
	}
	if item.Enabled {
		return "healthy"
	}
	return "disabled"
}

// catalogDisplayNames 批量取安装实例对应 catalog 行的真名（#46 批次 2）：
// 归一 extensionID → displayName（空则回退 name）；查表失败返回空 map，
// 调用侧回退 extensionID 原值，不阻塞列表。
func (s *Service) catalogDisplayNames(ctx context.Context, items []model.ExtensionInstallation) map[string]string {
	names := make(map[string]string)
	ids := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		id := normalizeExtensionID(item.ExtensionID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, item.ExtensionID)
	}
	if len(ids) == 0 {
		return names
	}
	rows, err := s.svcCtx.Extensions.Catalog.ListByExtensionIDs(ctx, ids)
	if err != nil {
		return names
	}
	for _, row := range rows {
		name := strings.TrimSpace(row.DisplayName)
		if name == "" {
			name = strings.TrimSpace(row.Name)
		}
		if name != "" {
			names[normalizeExtensionID(row.ExtensionID)] = name
		}
	}
	return names
}

func toEventItems(events []model.ExtensionEvent) []ExtensionEventItem {
	items := make([]ExtensionEventItem, 0, len(events))
	for _, event := range events {
		items = append(items, ExtensionEventItem{
			EventType: event.EventType,
			Level:     event.Level,
			Message:   event.Message,
			Payload:   string(event.PayloadJSON),
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

var allowedExtensionScopeTypes = map[string]struct{}{
	"system":     {},
	"global":     {},
	"game":       {},
	"env":        {},
	"node-group": {},
	"node":       {},
}

func normalizeAndValidateExtensionScope(scopeType, scopeID *string) error {
	normalizedType := strings.ToLower(strings.TrimSpace(*scopeType))
	normalizedID := strings.TrimSpace(*scopeID)
	*scopeType = normalizedType
	*scopeID = normalizedID

	if normalizedType == "" && normalizedID == "" {
		return nil
	}
	if normalizedType == "" || normalizedID == "" {
		return errorx.NewBadRequest("scope_type and scope_id must be provided together")
	}
	if _, ok := allowedExtensionScopeTypes[normalizedType]; !ok {
		return errorx.NewBadRequest("unsupported extension scope_type")
	}
	return nil
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
	// 按目标类型 uint 自身宽度解析（strconv.IntSize），uint(id) 转换在任意平台
	// 都可证明安全（依据同 id_helpers.go ParseUintID 注释）。
	if id, err := strconv.ParseUint(strings.TrimSpace(identifier), 10, strconv.IntSize); err == nil {
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
	// 设计债清理：parseDependencies 只在 ExtensionID TrimSpace 非空时产出条目
	// （string 与 map 两种形态同），原先的空 ID continue 防御恒假已删除（下同）。
	for _, dep := range deps {
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
	// 设计债清理：同 validateDependencies，parseDependencies 产物 ID 恒非空。
	for _, child := range children {
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
			raw = string(release.ManifestJSON)
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
			raw = string(r.ManifestJSON)
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

	// applyVersionOp 对未知操作符恒返 false（显式拒绝），直接传播即可。
	return applyVersionOp(op, current, target, cmp)
}

// applyVersionOp 按 op 对版本比较结果分派。抽为独立函数使未知操作符的
// 拒绝分支可被单测直接以任意 op 驱动（matchSingleClause 内部的 op 恒为
// 前缀提取的七种字面量之一，查表 miss 仅在 opMatchers 被裁剪时出现）。
func applyVersionOp(op string, current, target semVersion, cmp int) bool {
	fn, exists := opMatchers[op]
	if !exists {
		return false
	}
	return fn(current, target, cmp)
}

// opMatchers 是版本约束操作符的分派表。
var opMatchers = map[string]func(current, target semVersion, cmp int) bool{
	"=":  func(_, _ semVersion, cmp int) bool { return cmp == 0 },
	">":  func(_, _ semVersion, cmp int) bool { return cmp > 0 },
	">=": func(_, _ semVersion, cmp int) bool { return cmp >= 0 },
	"<":  func(_, _ semVersion, cmp int) bool { return cmp < 0 },
	"<=": func(_, _ semVersion, cmp int) bool { return cmp <= 0 },
	"^": func(current, target semVersion, _ int) bool {
		return matchCaretConstraint(current, target)
	},
	"~": func(current, target semVersion, _ int) bool {
		return matchTildeConstraint(current, target)
	},
}

func matchCaretConstraint(current, base semVersion) bool {
	if compareSemVersion(current, base) < 0 {
		return false
	}
	var upper semVersion
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
	var upper semVersion
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
