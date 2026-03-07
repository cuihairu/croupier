// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WorkspaceConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigLogic {
	return &WorkspaceConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListConfigs returns all workspace configs.
func (l *WorkspaceConfigLogic) ListConfigs(_ *types.WorkspaceConfigsListRequest) (*types.WorkspaceConfigsListResponse, error) {
	items, err := l.svcCtx.WorkspaceConfigModel.ListAll(l.ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]types.WorkspaceConfig, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i])
		_ = enrichWorkspaceVersion(l.ctx, l.svcCtx, &dto)
		dtos = append(dtos, dto)
	}
	return &types.WorkspaceConfigsListResponse{Items: dtos}, nil
}

// GetConfig returns a single workspace config by objectKey.
func (l *WorkspaceConfigLogic) GetConfig(req *types.WorkspaceConfigGetRequest) (*types.WorkspaceConfigGetResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	cfg, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	dto := toDTO(cfg)
	_ = enrichWorkspaceVersion(l.ctx, l.svcCtx, &dto)
	return &types.WorkspaceConfigGetResponse{WorkspaceConfig: dto}, nil
}

// SaveConfig creates or updates a workspace config.
func (l *WorkspaceConfigLogic) SaveConfig(req *types.WorkspaceConfigSaveRequest) (*types.WorkspaceConfigSaveResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}

	now := time.Now()

	// Try to load existing to preserve published state and timestamps.
	existing, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	var createdAt time.Time
	published := false
	var publishedAt *time.Time
	publishedBy := ""
	if err == nil {
		createdAt = existing.CreatedAt
		published = existing.Published
		publishedAt = existing.PublishedAt
		publishedBy = existing.PublishedBy
	} else {
		createdAt = now
	}

	// Build the full config JSON blob.
	meta := types.WorkspaceConfigMeta{
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: now.UTC().Format(time.RFC3339),
	}
	dto := types.WorkspaceConfig{
		ObjectKey:   req.ObjectKey,
		Title:       req.Title,
		Description: req.Description,
		Layout:      req.Layout,
		Published:   published,
		MenuOrder:   req.MenuOrder,
		Meta:        meta,
	}
	if publishedAt != nil {
		dto.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	}
	dto.PublishedBy = publishedBy
	if req.Status != "" {
		dto.Status = req.Status
	}
	dto.Status = resolveWorkspaceStatus(&dto)

	configJSON, err := json.Marshal(dto)
	if err != nil {
		return nil, errorx.NewInternalError("failed to marshal workspace config")
	}

	record := &model.WorkspaceConfig{
		ObjectKey:   req.ObjectKey,
		Title:       req.Title,
		Published:   published,
		PublishedAt: publishedAt,
		PublishedBy: publishedBy,
		MenuOrder:   req.MenuOrder,
		Config:      configJSON,
	}

	if err := l.svcCtx.WorkspaceConfigModel.Upsert(l.ctx, record); err != nil {
		return nil, err
	}

	return &types.WorkspaceConfigSaveResponse{WorkspaceConfig: dto}, nil
}

// DeleteConfig removes a workspace config by objectKey.
func (l *WorkspaceConfigLogic) DeleteConfig(req *types.WorkspaceConfigDeleteRequest) (*types.WorkspaceConfigDeleteResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	_, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	if err := l.svcCtx.WorkspaceConfigModel.Delete(l.ctx, req.ObjectKey); err != nil {
		return nil, err
	}
	return &types.WorkspaceConfigDeleteResponse{Message: "deleted"}, nil
}

// Publish marks a workspace config as published.
func (l *WorkspaceConfigLogic) Publish(req *types.WorkspacePublishRequest) (*types.WorkspacePublishResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	_, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	if err := l.svcCtx.WorkspaceConfigModel.SetPublished(l.ctx, req.ObjectKey, true, req.PublishedBy); err != nil {
		return nil, err
	}
	return &types.WorkspacePublishResponse{Published: true, ObjectKey: req.ObjectKey}, nil
}

// Unpublish marks a workspace config as unpublished.
func (l *WorkspaceConfigLogic) Unpublish(req *types.WorkspaceUnpublishRequest) (*types.WorkspaceUnpublishResponse, error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	_, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	if err := l.svcCtx.WorkspaceConfigModel.SetPublished(l.ctx, req.ObjectKey, false, ""); err != nil {
		return nil, err
	}
	return &types.WorkspaceUnpublishResponse{Published: false, ObjectKey: req.ObjectKey}, nil
}

// ListPublished returns all published workspace configs.
func (l *WorkspaceConfigLogic) ListPublished(_ *types.WorkspacePublishedListRequest) (*types.WorkspacePublishedListResponse, error) {
	items, err := l.svcCtx.WorkspaceConfigModel.ListPublished(l.ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]types.WorkspaceConfig, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i])
		_ = enrichWorkspaceVersion(l.ctx, l.svcCtx, &dto)
		dtos = append(dtos, dto)
	}
	return &types.WorkspacePublishedListResponse{Items: dtos}, nil
}

// toDTO converts a model.WorkspaceConfig to a types.WorkspaceConfig.
func toDTO(m *model.WorkspaceConfig) types.WorkspaceConfig {
	dto := types.WorkspaceConfig{
		ObjectKey: m.ObjectKey,
		Title:     m.Title,
		Published: m.Published,
		MenuOrder: m.MenuOrder,
		Meta: types.WorkspaceConfigMeta{
			CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339),
		},
	}
	if m.PublishedAt != nil {
		dto.PublishedAt = m.PublishedAt.UTC().Format(time.RFC3339)
	}
	dto.PublishedBy = m.PublishedBy

	// Unmarshal the full config blob to recover Layout.
	if len(m.Config) > 0 {
		var full types.WorkspaceConfig
		if err := json.Unmarshal(m.Config, &full); err == nil {
			dto.Layout = full.Layout
			dto.Description = full.Description
			if strings.TrimSpace(full.Status) != "" {
				dto.Status = strings.TrimSpace(full.Status)
			}
		}
	}
	dto.Status = resolveWorkspaceStatus(&dto)
	return dto
}
