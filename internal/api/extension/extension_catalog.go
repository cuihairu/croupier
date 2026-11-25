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

// ---- catalog write path (#46 batch 3): register / update (incl. listing
// status) / remove / publish release ----

// catalogStatuses: closed set of listing statuses (delisted = delisted; list
// and install entry points filter by status).
var catalogStatuses = map[string]bool{"active": true, "delisted": true}

// catalogReleaseChannels: closed set of release channels.
var catalogReleaseChannels = map[string]bool{"stable": true, "beta": true, "alpha": true}

// validateCatalogExtensionID enforces the extension id shape: must start with
// a lowercase letter or digit, allows . _ - to stay compatible with the
// official.<domain> naming rule, and forbids whitespace to avoid route
// parameter ambiguity.
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

// catalogItemFromModel assembles the write-path response body (reuses the read
// side item shape; does not derive the installed flag).
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
	_ = operator // auditing goes through the unified HTTP-layer audit chain; the catalog row has no createdBy column
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
	// Reject removal while active installations exist (uninstall first, then
	// remove, to avoid dangling references).
	if err := s.rejectActiveInstallations(ctx, existing.ExtensionID); err != nil {
		return err
	}
	if err := s.svcCtx.Extensions.Catalog.RemoveReleases(ctx, existing.ExtensionID); err != nil {
		return mapServiceError(err)
	}
	return mapServiceError(s.svcCtx.Extensions.Catalog.Remove(ctx, existing.ExtensionID))
}

// rejectActiveInstallations rejects destructive operations other than catalog
// removal/delisting while uninstall-pending installations exist.
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
	// latestVersion semantics: only bump the catalog pointer when the new
	// version has a higher semver (re-publishing on a lower channel does not
	// roll latest back).
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

// extensionPackMaxSize is the byte limit for a single pack(.tgz) file (64 MiB).
const extensionPackMaxSize = 64 << 20

// extensionPackManifestMaxSize is the byte limit for manifest.json inside the
// pack (1 MiB).
const extensionPackManifestMaxSize = 1 << 20

// extensionPackManifest is the parse target for manifest.json inside a pack.
// Field semantics match the catalog register / release publish requests
// (extensionId/version/manifest required); the remaining fields fall back
// through the same defaulting chain as their respective write paths.
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

// parseExtensionPack unpacks a .tgz and extracts manifest.json. The manifest
// may sit at the pack root or inside a single top-level directory (both the
// common tar czf ./ prefix and a builddir prefix are tolerated); with nested
// layers the shallowest copy wins. Non-gzip/tar input, a missing manifest, or
// malformed manifest JSON all return a 400 semantic error.
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

// PackImport handles POST /extensions/packs/import: upload .tgz -> extract
// manifest.json -> auto-register in the catalog (reusing an existing
// registration) -> publish the release (version dedupe returns 409) -> write
// the artifact to object storage (sha256 computed server-side). The catalog
// closed-set/shape validations share the same rules as the manual write path;
// other files in the pack are not parsed and are stored as part of the
// artifact.
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

	// Catalog registration: reuse an existing row (importing a newer version
	// does not change the registration metadata).
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
	// latestVersion semantics match CatalogReleasePublish: only a higher
	// semver back-fills.
	if shouldBumpLatestVersion(existing.LatestVersion, parsed) {
		if err := s.svcCtx.Extensions.Catalog.UpdateFields(ctx, extID, map[string]any{"latest_version": version}); err != nil {
			return nil, mapServiceError(err)
		}
		existing.LatestVersion = version
	}
	_ = operator // auditing goes through the unified HTTP-layer audit chain
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

// shouldBumpLatestVersion back-fills when the catalog has no latest version or
// the new version has a strictly higher semver.
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
