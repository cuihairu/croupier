// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存 Workspace 配置（创建或更新）
func NewWorkspaceConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigSaveLogic {
	return &WorkspaceConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigSaveLogic) WorkspaceConfigSave(req *types.WorkspaceConfigSaveRequest) (resp *types.WorkspaceConfigSaveResponse, err error) {
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
		ObjectKey: req.ObjectKey,
		Title:     req.Title,
		Layout:    req.Layout,
		Published: published,
		MenuOrder: req.MenuOrder,
		Meta:      meta,
	}
	if publishedAt != nil {
		dto.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	}
	dto.PublishedBy = publishedBy

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
