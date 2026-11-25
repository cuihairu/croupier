package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"gorm.io/gorm"
)

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
	// ARCH-021: restricted machine identities are authorized by scope -- a
	// machine token is bound to a single agentID, so requesting another
	// agent's payload yields 403 (cross-scope rejection); admin identities
	// still go through RBAC read permission, so server-side authorization is
	// not relaxed just because the caller is a machine.
	if claims, ok := jwtutil.MachineIdentityFrom(ctx); ok {
		if claims.AgentID != strings.TrimSpace(agentID) {
			return nil, errorx.NewForbidden("agent scope mismatch")
		}
	} else if err := s.requireReadPermission(ctx, "无权查看扩展同步数据"); err != nil {
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

// deriveExtensionHealthStatus uses the same criteria as HealthCheck (no health
// probe is persisted; derived from status/enabled): uninstalled (status or
// desired_state) -> uninstalled; enabled -> healthy; otherwise disabled.
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

// catalogDisplayNames batch-fetches the display names of the catalog rows
// behind the given installations (#46 batch 2): normalized extensionID ->
// displayName (falling back to name when empty); a lookup failure returns an
// empty map and callers fall back to the raw extensionID without blocking the
// list.
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
	// Design-debt cleanup: parseDependencies only yields entries when the
	// TrimSpace'd ExtensionID is non-empty (true for both the string and map
	// shapes), so the former defensive continue on an empty ID, which could
	// never fire, has been removed (same below).
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
	// Design-debt cleanup: as in validateDependencies, the IDs produced by
	// parseDependencies are never empty.
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
