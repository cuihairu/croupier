package extension

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

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

// extensionPackMaxSize 是单个 pack(.tgz) 的字节上限（64 MiB）。
const extensionPackMaxSize = 64 << 20

// extensionPackManifestMaxSize 是包内 manifest.json 的字节上限（1 MiB）。
const extensionPackManifestMaxSize = 1 << 20

// extensionPackManifest 是 pack 内 manifest.json 的解析目标。字段语义与
// catalog 登记 / release 发布请求一致（extensionId/version/manifest 必填），
// 其余缺省走各自写路径的同一套兜底链。
type extensionPackManifest struct {
	ExtensionID    string          `json:"extensionId"`
	Name           string          `json:"name"`
	DisplayName    string          `json:"displayName"`
	Vendor         string          `json:"vendor"`
	Kind           string          `json:"kind"`
	Summary        string          `json:"summary"`
	IconURL        string          `json:"iconUrl"`
	HomepageURL    string          `json:"homepageUrl"`
	Version        string          `json:"version"`
	ReleaseChannel string          `json:"releaseChannel"`
	MinCoreVersion string          `json:"minCoreVersion"`
	Changelog      string          `json:"changelog"`
	Manifest       json.RawMessage `json:"manifest"`
}

// parseExtensionPack 解包 .tgz 并取出 manifest.json。包根或单一顶层目录下
// 的 manifest.json 均可（tar czf 常见的 ./ 前缀与 builddir 前缀都容忍）；
// 多层嵌套取最浅一份。非 gzip/tar、缺 manifest、manifest 坏 JSON 均报
// 400 语义错误。
func parseExtensionPack(data []byte) (*extensionPackManifest, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errorx.NewBadRequest("pack 不是有效的 gzip 流")
	}
	tr := tar.NewReader(gz)
	var found *extensionPackManifest
	bestDepth := -1
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errorx.NewBadRequest("pack 不是有效的 tar 归档")
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.Split(name, "/")
		if parts[len(parts)-1] != "manifest.json" {
			continue
		}
		depth := len(parts) - 1
		if bestDepth >= 0 && depth >= bestDepth {
			continue
		}
		if hdr.Size > extensionPackManifestMaxSize {
			return nil, errorx.NewBadRequest("manifest.json 超过大小上限 1MiB")
		}
		raw, err := io.ReadAll(io.LimitReader(tr, extensionPackManifestMaxSize+1))
		if err != nil {
			return nil, errorx.NewBadRequest("读取 manifest.json 失败")
		}
		var m extensionPackManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, errorx.NewBadRequest("manifest.json 不是有效 JSON")
		}
		found, bestDepth = &m, depth
	}
	if found == nil {
		return nil, errorx.NewBadRequest("pack 缺少 manifest.json")
	}
	return found, nil
}

// PackImport 处理 POST /extensions/packs/import：上传 .tgz → 解出
// manifest.json → 自动登记 catalog（已存在则复用）→ 发布 release（版本查重
// 409）→ 工件写入对象存储（sha256 服务端计算）。非 catalog 闭集/形态校验
// 与手填写路径共用同一套规则；包内其余文件不解析，仅随工件整体存储。
func (s *Service) PackImport(ctx context.Context, data io.Reader, size int64, operator string) (*ExtensionPackImportResponse, error) {
	if err := s.requireWritePermission(ctx, "无权导入扩展包"); err != nil {
		return nil, err
	}
	if s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewBadRequest("对象存储未配置")
	}
	if size > extensionPackMaxSize {
		return nil, errorx.NewBadRequest(fmt.Sprintf("pack 超过大小上限 %dMiB", extensionPackMaxSize>>20))
	}
	buf, err := io.ReadAll(io.LimitReader(data, extensionPackMaxSize+1))
	if err != nil {
		return nil, errorx.NewBadRequest("读取上传内容失败")
	}
	if int64(len(buf)) > extensionPackMaxSize {
		return nil, errorx.NewBadRequest(fmt.Sprintf("pack 超过大小上限 %dMiB", extensionPackMaxSize>>20))
	}
	digest := sha256.Sum256(buf)
	checksum := "sha256:" + hex.EncodeToString(digest[:])

	m, err := parseExtensionPack(buf)
	if err != nil {
		return nil, err
	}
	extID := strings.TrimSpace(m.ExtensionID)
	if err := validateCatalogExtensionID(extID); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(m.Version)
	parsed, ok := parseSemVersion(version)
	if !ok {
		return nil, errorx.NewBadRequest("invalid version: expected semver like 1.2.3")
	}
	channel := strings.TrimSpace(m.ReleaseChannel)
	if channel == "" {
		channel = "stable"
	}
	if !catalogReleaseChannels[channel] {
		return nil, errorx.NewBadRequest("invalid releaseChannel: only stable/beta/alpha allowed")
	}
	var manifestObj map[string]any
	if err := json.Unmarshal(m.Manifest, &manifestObj); err != nil || manifestObj == nil {
		return nil, errorx.NewBadRequest("manifest 必须为 JSON 对象")
	}
	manifestJSON, err := json.Marshal(manifestObj)
	if err != nil {
		return nil, errorx.NewBadRequest("manifest 必须为 JSON 对象")
	}

	// catalog 登记：已存在则复用（导入迭代版本不改登记元数据）。
	existing, _, err := s.svcCtx.Extensions.Catalog.Get(ctx, extID)
	catalogCreated := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = extID
		}
		displayName := strings.TrimSpace(m.DisplayName)
		if displayName == "" {
			displayName = name
		}
		vendor := strings.TrimSpace(m.Vendor)
		if vendor == "" {
			vendor = "external"
		}
		kind := strings.TrimSpace(m.Kind)
		if kind == "" {
			kind = "community"
		}
		existing = &model.ExtensionCatalog{
			ExtensionID:   extID,
			Name:          name,
			DisplayName:   displayName,
			Vendor:        vendor,
			Kind:          kind,
			Summary:       strings.TrimSpace(m.Summary),
			IconURL:       strings.TrimSpace(m.IconURL),
			HomepageURL:   strings.TrimSpace(m.HomepageURL),
			Status:        "active",
			LatestVersion: version,
		}
		if err := s.svcCtx.Extensions.Catalog.Create(ctx, existing); err != nil {
			return nil, mapServiceError(err)
		}
		catalogCreated = true
	} else if err != nil {
		return nil, mapServiceError(err)
	}

	if _, dup := s.svcCtx.Extensions.Catalog.ReleaseByVersion(ctx, extID, version); dup == nil {
		return nil, errorx.NewConflict("release version already exists")
	} else if !errors.Is(dup, gorm.ErrRecordNotFound) {
		return nil, mapServiceError(dup)
	}

	packageRef := fmt.Sprintf("extension-packs/%s/%s.tgz", extID, version)
	if err := s.svcCtx.ObjectStore.Put(ctx, packageRef, bytes.NewReader(buf), int64(len(buf)), "application/gzip"); err != nil {
		return nil, fmt.Errorf("写入对象存储失败: %w", err)
	}
	release := &model.ExtensionRelease{
		ExtensionID:     extID,
		Version:         version,
		ReleaseChannel:  channel,
		ManifestJSON:    model.JSON(manifestJSON),
		PackageRef:      packageRef,
		Checksum:        checksum,
		MinCoreVersion:  strings.TrimSpace(m.MinCoreVersion),
		Changelog:       strings.TrimSpace(m.Changelog),
		PublishedAtUnix: time.Now().Unix(),
	}
	if err := s.svcCtx.Extensions.Catalog.PublishRelease(ctx, release); err != nil {
		return nil, mapServiceError(err)
	}
	// latestVersion 语义与 CatalogReleasePublish 一致：仅更高 semver 回填。
	if shouldBumpLatestVersion(existing.LatestVersion, parsed) {
		if err := s.svcCtx.Extensions.Catalog.UpdateFields(ctx, extID, map[string]any{"latest_version": version}); err != nil {
			return nil, mapServiceError(err)
		}
		existing.LatestVersion = version
	}
	_ = operator // 审计经 HTTP 层统一审计链
	return &ExtensionPackImportResponse{
		Catalog: catalogItemFromModel(existing),
		Release: ExtensionReleaseItem{
			Version:        release.Version,
			ReleaseChannel: release.ReleaseChannel,
			MinCoreVersion: release.MinCoreVersion,
			PublishedAt:    release.PublishedAtUnix,
			Changelog:      release.Changelog,
		},
		CatalogCreated: catalogCreated,
		PackageRef:     packageRef,
		Checksum:       checksum,
		Size:           int64(len(buf)),
	}, nil
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
