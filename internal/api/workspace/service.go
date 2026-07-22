package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
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

// ListConfigs returns all workspace configurations
func (s *Service) ListConfigs(ctx context.Context, req *ListConfigsRequest) (*ListConfigsResponse, error) {
	items, err := s.svcCtx.WorkspaceConfigModel.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]WorkspaceConfig, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i])
		_ = enrichWorkspaceVersion(ctx, s.svcCtx, &dto)
		dtos = append(dtos, dto)
	}
	return &ListConfigsResponse{Items: dtos}, nil
}

// ListPublished returns published workspace configurations
func (s *Service) ListPublished(ctx context.Context, req *ListPublishedRequest) (*ListPublishedResponse, error) {
	items, err := s.svcCtx.WorkspaceConfigModel.ListPublished(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]WorkspaceConfig, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i])
		_ = enrichWorkspaceVersion(ctx, s.svcCtx, &dto)
		dtos = append(dtos, dto)
	}
	return &ListPublishedResponse{Items: dtos}, nil
}

// GetConfig returns a workspace configuration by object key.
// If the config does not exist, a minimal default is auto-created so the UI
// editor has a starting point instead of returning 404.
func (s *Service) GetConfig(ctx context.Context, req *GetConfigRequest) (*GetConfigResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	cfg, err := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.createDefaultConfig(ctx, req.ObjectKey)
		}
		return nil, err
	}
	dto := toDTO(cfg)
	_ = enrichWorkspaceVersion(ctx, s.svcCtx, &dto)
	return &GetConfigResponse{WorkspaceConfig: dto}, nil
}

// createDefaultConfig creates a minimal default workspace config (empty tabs
// layout) and persists it, so subsequent accesses find an existing record.
func (s *Service) createDefaultConfig(ctx context.Context, objectKey string) (*GetConfigResponse, error) {
	defaultLayout := map[string]any{
		"type": "tabs",
		"tabs": []any{},
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// Store the full WorkspaceConfig JSON so toDTO can parse it uniformly.
	defaultCfg := WorkspaceConfig{
		ObjectKey: objectKey,
		Title:     objectKey,
		Layout:    defaultLayout,
		Status:    workspaceStatusDraft,
		Meta: WorkspaceConfigMeta{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	configJSON, err := json.Marshal(defaultCfg)
	if err != nil {
		return nil, errorx.NewInternalError("failed to marshal default workspace config")
	}
	record := &model.WorkspaceConfig{
		ObjectKey: objectKey,
		Title:     objectKey,
		Config:    string(configJSON),
	}
	if err := s.svcCtx.WorkspaceConfigModel.Upsert(ctx, record); err != nil {
		return nil, err
	}
	defaultCfg.CreatedAt = now
	defaultCfg.UpdatedAt = now
	return &GetConfigResponse{WorkspaceConfig: defaultCfg}, nil
}

func (s *Service) saveWorkspaceDTO(ctx context.Context, dto WorkspaceConfig) error {
	if strings.TrimSpace(dto.ObjectKey) == "" {
		return errorx.NewBadRequest("objectKey is required")
	}
	var publishedAt *time.Time
	if dto.Published {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(dto.PublishedAt))
		if err != nil {
			return errorx.NewBadRequest("publishedAt must be RFC3339 when workspace is published")
		}
		publishedAt = &parsed
	}
	stored := WorkspaceConfig{
		ObjectKey:   dto.ObjectKey,
		Title:       dto.Title,
		Description: dto.Description,
		Layout:      dto.Layout,
		Published:   dto.Published,
		PublishedBy: dto.PublishedBy,
		MenuOrder:   dto.MenuOrder,
		Status:      resolveWorkspaceStatus(&dto),
		Category:    dto.Category,
		Permissions: dto.Permissions,
		Meta: WorkspaceConfigMeta{
			CreatedAt: dto.CreatedAt,
			UpdatedAt: dto.UpdatedAt,
		},
		Version: dto.Version,
	}
	if dto.PublishedAt != "" {
		stored.PublishedAt = dto.PublishedAt
	}

	configJSON, err := json.Marshal(stored)
	if err != nil {
		return errorx.NewInternalError("failed to marshal workspace config")
	}

	record := &model.WorkspaceConfig{
		ObjectKey:   dto.ObjectKey,
		Title:       dto.Title,
		Published:   dto.Published,
		PublishedAt: publishedAt,
		PublishedBy: dto.PublishedBy,
		MenuOrder:   dto.MenuOrder,
		Config:      string(configJSON),
	}

	return s.svcCtx.WorkspaceConfigModel.Upsert(ctx, record)
}

// SaveConfig saves (creates or updates) a workspace configuration
func (s *Service) SaveConfig(ctx context.Context, req *SaveConfigRequest) (*SaveConfigResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}

	now := time.Now()

	// Load existing and merge (partial update semantics): keep untouched fields as-is.
	existing, err := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, req.ObjectKey)
	published := req.Published
	publishedBy := req.PublishedBy
	var publishedAt *time.Time
	exists := err == nil
	if exists {
		// 如果是更新，保留已发布的状态（除非请求中明确指定了）
		if !req.Published {
			published = existing.Published
			publishedBy = existing.PublishedBy
		}
		publishedAt = existing.PublishedAt
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// New record
	} else {
		return nil, err
	}

	var dto WorkspaceConfig
	if exists {
		dto = toDTO(existing)
	} else {
		dto = WorkspaceConfig{
			ObjectKey: req.ObjectKey,
			Title:     req.ObjectKey,
			MenuOrder: 0,
			CreatedAt: now.UTC().Format(time.RFC3339),
		}
	}

	// Apply patch-style updates from request.
	dto.ObjectKey = req.ObjectKey
	if title := strings.TrimSpace(req.Title); title != "" {
		dto.Title = title
	}
	if desc := strings.TrimSpace(req.Description); desc != "" {
		dto.Description = desc
	}
	if req.Layout != nil {
		dto.Layout = req.Layout
	}
	if req.MenuOrder != 0 || !exists {
		dto.MenuOrder = req.MenuOrder
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		dto.Status = status
	}
	dto.Category = strings.TrimSpace(req.Category)
	dto.Permissions = req.Permissions

	// Validate minimal required fields after merge.
	if strings.TrimSpace(dto.Title) == "" {
		return nil, errorx.NewBadRequest("title is required")
	}
	if dto.Layout == nil {
		return nil, errorx.NewBadRequest("layout is required")
	}

	dto.UpdatedAt = now.UTC().Format(time.RFC3339)
	dto.Published = published
	if publishedAt != nil {
		dto.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	}
	dto.PublishedBy = publishedBy
	dto.Status = resolveWorkspaceStatus(&dto)

	if err := s.saveWorkspaceDTO(ctx, dto); err != nil {
		return nil, err
	}

	actor := workspaceActorFromCtx(ctx)
	if version, versionErr := persistWorkspaceVersion(
		ctx,
		s.svcCtx,
		dto,
		actor,
		"save workspace config",
	); versionErr == nil {
		dto.Version = version
	}

	return &SaveConfigResponse{WorkspaceConfig: dto}, nil
}

// DeleteConfig deletes a workspace configuration
func (s *Service) DeleteConfig(ctx context.Context, req *DeleteConfigRequest) (*DeleteConfigResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	_, err := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	if err := s.svcCtx.WorkspaceConfigModel.Delete(ctx, req.ObjectKey); err != nil {
		return nil, err
	}
	return &DeleteConfigResponse{Message: "deleted"}, nil
}

// Publish publishes a workspace configuration
func (s *Service) Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	current, err := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	dto := toDTO(current)
	if err := validateWorkspaceForPublish(dto); err != nil {
		return nil, err
	}
	actor := strings.TrimSpace(req.PublishedBy)
	if actor == "" {
		actor = workspaceActorFromCtx(ctx)
	}
	publishedAt := time.Now()
	dto.Published = true
	dto.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	dto.PublishedBy = actor
	dto.Status = workspaceStatusPublished
	dto.UpdatedAt = publishedAt.UTC().Format(time.RFC3339)

	if err := s.saveWorkspaceDTO(ctx, dto); err != nil {
		return nil, err
	}
	_, _ = persistWorkspaceVersion(ctx, s.svcCtx, dto, actor, "publish workspace config")
	return &PublishResponse{Published: true, ObjectKey: req.ObjectKey}, nil
}

// Unpublish unpublishes a workspace configuration
func (s *Service) Unpublish(ctx context.Context, req *UnpublishRequest) (*UnpublishResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	current, err := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	dto := toDTO(current)
	now := time.Now()
	dto.Published = false
	dto.PublishedAt = ""
	dto.PublishedBy = ""
	dto.Status = workspaceStatusDraft
	dto.UpdatedAt = now.UTC().Format(time.RFC3339)

	if err := s.saveWorkspaceDTO(ctx, dto); err != nil {
		return nil, err
	}

	actor := workspaceActorFromCtx(ctx)
	_, _ = persistWorkspaceVersion(ctx, s.svcCtx, dto, actor, "unpublish workspace config")
	return &UnpublishResponse{Published: false, ObjectKey: req.ObjectKey}, nil
}

// Versions returns workspace version history
func (s *Service) Versions(ctx context.Context, req *VersionsRequest) (*VersionsResponse, error) {
	records, err := s.listVersions(ctx, req.ObjectKey, req.From, req.To)
	if err != nil {
		return nil, err
	}

	return &VersionsResponse{
		Items: records,
	}, nil
}

// VersionDetail returns a specific workspace version
func (s *Service) VersionDetail(ctx context.Context, req *VersionDetailRequest) (*VersionDetailResponse, error) {
	// Parse version from VersionID (could be "v1", "1", etc.)
	versionID := strings.TrimSpace(req.VersionID)
	versionID = strings.TrimPrefix(versionID, "v")
	version, err := strconv.Atoi(versionID)
	if err != nil {
		return nil, errors.New("invalid version ID")
	}

	// Get the workspace version key
	key := workspaceVersionKey(req.ObjectKey)

	// Find the specific version record
	record, err := s.svcCtx.ConfigVersionModel.Find(ctx, key, version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("workspace version not found")
		}
		return nil, err
	}

	// Parse the config JSON
	var configData interface{}
	if record.Value != "" {
		if err := json.Unmarshal([]byte(record.Value), &configData); err != nil {
			// If JSON parsing fails, return the raw string
			configData = record.Value
		}
	}

	// Build the response record
	versionRecord := WorkspaceVersionRecord{
		ID:        strconv.FormatUint(uint64(record.ID), 10),
		ObjectKey: record.Key,
		Version:   record.Version,
		Config:    configData,
		CreatedAt: record.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy: record.CreatedBy,
		Comment:   record.Message,
	}

	// Check if this is the current draft or published version
	latest, err := s.svcCtx.ConfigVersionModel.FindLatest(ctx, key)
	if err == nil {
		versionRecord.IsCurrentDraft = (latest.ID == record.ID)
	}

	return &VersionDetailResponse{
		WorkspaceVersionRecord: versionRecord,
	}, nil
}

// Rollback rolls back a workspace to a specific version
func (s *Service) Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	objectKey := strings.TrimSpace(req.ObjectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	versionID := strings.TrimSpace(req.VersionID)
	if versionID == "" {
		return nil, errorx.NewBadRequest("versionId is required")
	}
	version, err := strconv.Atoi(versionID)
	if err != nil || version <= 0 {
		return nil, errorx.NewBadRequest("invalid versionId")
	}

	record, err := s.svcCtx.ConfigVersionModel.Find(ctx, workspaceVersionKey(objectKey), version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace version not found")
		}
		return nil, err
	}

	// Parse the workspace config from the version record
	var workspaceCfg WorkspaceConfig
	if err := json.Unmarshal([]byte(record.Value), &workspaceCfg); err != nil {
		return nil, errorx.NewInternalError("failed to parse workspace config from version")
	}

	if s.svcCtx.WorkspaceConfigModel == nil {
		return nil, errorx.NewInternalError("workspace config model not available")
	}
	now := time.Now()
	workspaceCfg.ObjectKey = objectKey
	if strings.TrimSpace(workspaceCfg.Title) == "" {
		workspaceCfg.Title = objectKey
	}
	workspaceCfg.UpdatedAt = now.UTC().Format(time.RFC3339)
	workspaceCfg.Status = resolveWorkspaceStatus(&workspaceCfg)
	if workspaceCfg.Published {
		if workspaceCfg.PublishedAt == "" {
			workspaceCfg.PublishedAt = now.UTC().Format(time.RFC3339)
		}
		if workspaceCfg.PublishedBy == "" {
			workspaceCfg.PublishedBy = workspaceActorFromCtx(ctx)
		}
	}

	if err := s.saveWorkspaceDTO(ctx, workspaceCfg); err != nil {
		return nil, err
	}

	return &RollbackResponse{
		ObjectKey: objectKey,
		Version:   version,
	}, nil
}

// listVersions returns the list of workspace versions
func (s *Service) listVersions(ctx context.Context, objectKey, from, to string) ([]WorkspaceVersionRecord, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	fromAt, toAt, err := parseWorkspaceVersionTimeRange(from, to)
	if err != nil {
		return nil, err
	}
	versionKey := workspaceVersionKey(objectKey)
	records, err := s.svcCtx.ConfigVersionModel.List(ctx, versionKey)
	if err != nil {
		return nil, err
	}
	currentDraftVersion := 0
	if latest, latestErr := s.svcCtx.ConfigVersionModel.FindLatest(ctx, versionKey); latestErr == nil {
		currentDraftVersion = latest.Version
	} else if latestErr != nil && !errors.Is(latestErr, gorm.ErrRecordNotFound) {
		return nil, latestErr
	}

	currentPublishedVersion := 0
	if latestCfg, cfgErr := s.svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, objectKey); cfgErr == nil {
		if latestCfg.Published && latestCfg.PublishedAt != nil {
			// Find the version that matches the published_at timestamp
			publishedTime := latestCfg.PublishedAt.Unix()
			for _, rec := range records {
				if rec.CreatedAt.Unix() == publishedTime {
					currentPublishedVersion = rec.Version
					break
				}
			}
		}
	}

	result := make([]WorkspaceVersionRecord, 0, len(records))
	for _, record := range records {
		// Apply time range filter if specified
		if !fromAt.IsZero() && record.CreatedAt.Before(fromAt) {
			continue
		}
		if !toAt.IsZero() && record.CreatedAt.After(toAt) {
			continue
		}

		var configData interface{}
		if record.Value != "" {
			if err := json.Unmarshal([]byte(record.Value), &configData); err != nil {
				configData = record.Value // fallback to raw string
			}
		}

		result = append(result, WorkspaceVersionRecord{
			ID:                 strconv.FormatUint(uint64(record.ID), 10),
			ObjectKey:          record.Key,
			Version:            record.Version,
			Config:             configData,
			IsCurrentDraft:     record.Version == currentDraftVersion,
			IsCurrentPublished: record.Version == currentPublishedVersion,
			CreatedAt:          record.CreatedAt.Format(time.RFC3339),
			CreatedBy:          record.CreatedBy,
			Comment:            record.Message,
		})
	}

	return result, nil
}
