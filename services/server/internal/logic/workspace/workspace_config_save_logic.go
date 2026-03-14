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

	"gorm.io/gorm"
)

type WorkspaceConfigSaveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存 Workspace 配置（创建或更新）
func NewWorkspaceConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigSaveLogic {
	return &WorkspaceConfigSaveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigSaveLogic) WorkspaceConfigSave(req *types.WorkspaceConfigSaveRequest) (resp *types.WorkspaceConfigSaveResponse, err error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}

	now := time.Now()

	// Load existing and merge (partial update semantics): keep untouched fields as-is.
	existing, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	var createdAt time.Time
	published := false
	var publishedAt *time.Time
	publishedBy := ""
	exists := err == nil
	if exists {
		createdAt = existing.CreatedAt
		published = existing.Published
		publishedAt = existing.PublishedAt
		publishedBy = existing.PublishedBy
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		createdAt = now
	} else {
		return nil, err
	}

	var dto types.WorkspaceConfig
	if exists {
		dto = toDTO(existing)
	} else {
		dto = types.WorkspaceConfig{
			ObjectKey: req.ObjectKey,
			Title:     req.ObjectKey,
			MenuOrder: 0,
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

	// Validate minimal required fields after merge.
	if strings.TrimSpace(dto.Title) == "" {
		return nil, errorx.NewBadRequest("title is required")
	}
	if dto.Layout == nil {
		return nil, errorx.NewBadRequest("layout is required")
	}

	meta := types.WorkspaceConfigMeta{
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: now.UTC().Format(time.RFC3339),
	}
	dto.Published = published
	dto.Meta = meta
	if publishedAt != nil {
		dto.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	}
	dto.PublishedBy = publishedBy
	dto.Status = resolveWorkspaceStatus(&dto)

	configJSON, err := json.Marshal(dto)
	if err != nil {
		return nil, errorx.NewInternalError("failed to marshal workspace config")
	}

	record := &model.WorkspaceConfig{
		ObjectKey:   req.ObjectKey,
		Title:       dto.Title,
		Published:   published,
		PublishedAt: publishedAt,
		PublishedBy: publishedBy,
		MenuOrder:   dto.MenuOrder,
		Config:      configJSON,
	}

	if err := l.svcCtx.WorkspaceConfigModel.Upsert(l.ctx, record); err != nil {
		return nil, err
	}
	actor := workspaceActorFromCtx(l.ctx)
	if version, versionErr := persistWorkspaceVersion(
		l.ctx,
		l.svcCtx,
		dto,
		actor,
		"save workspace config",
	); versionErr == nil {
		dto.Version = version
	}
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.save", req.ObjectKey, "success", map[string]interface{}{
		"title":     dto.Title,
		"status":    dto.Status,
		"menuOrder": dto.MenuOrder,
	})

	return &types.WorkspaceConfigSaveResponse{WorkspaceConfig: dto}, nil
}
