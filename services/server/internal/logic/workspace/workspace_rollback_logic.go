// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WorkspaceRollbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 回滚 Workspace 到指定版本
func NewWorkspaceRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceRollbackLogic {
	return &WorkspaceRollbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceRollbackLogic) WorkspaceRollback(req *types.WorkspaceRollbackRequest) (*types.WorkspaceRollbackResponse, error) {
	objectKey := strings.TrimSpace(req.ObjectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	versionID := strings.TrimSpace(req.VersionId)
	if versionID == "" {
		return nil, errorx.NewBadRequest("versionId is required")
	}
	version, err := strconv.Atoi(versionID)
	if err != nil || version <= 0 {
		return nil, errorx.NewBadRequest("invalid versionId")
	}

	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, workspaceVersionKey(objectKey), version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace version not found")
		}
		return nil, err
	}

	// Parse the workspace config from the version record
	var workspaceCfg types.WorkspaceConfig
	if err := json.Unmarshal([]byte(record.Value), &workspaceCfg); err != nil {
		return nil, errorx.NewInternalError("failed to parse workspace config from version")
	}

	// Apply the rollback: update the workspace config with the version data
	cfgModel := l.svcCtx.WorkspaceConfigModel
	if cfgModel == nil {
		return nil, errorx.NewInternalError("workspace config model not available")
	}

	// Marshal just the layout part to store in Config field
	layoutJSON, err := json.Marshal(workspaceCfg.Layout)
	if err != nil {
		return nil, errorx.NewInternalError("failed to marshal layout: " + err.Error())
	}

	// Update the workspace config with the rolled-back data
	update := &model.WorkspaceConfig{
		ObjectKey: objectKey,
		Title:     workspaceCfg.Title,
		Config:    datatypes.JSON(layoutJSON),
	}

	if err := cfgModel.Upsert(l.ctx, update); err != nil {
		return nil, err
	}

	// Append audit entry for rollback
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.rollback", objectKey, "success", map[string]interface{}{
		"version": version,
		"actor":   workspaceActorFromCtx(l.ctx),
	})

	// If the rolled-back version was published, also mark it as published
	if workspaceCfg.Published {
		actor := workspaceActorFromCtx(l.ctx)
		if err := cfgModel.SetPublished(l.ctx, objectKey, true, actor); err != nil {
			return nil, err
		}
	}

	return &types.WorkspaceRollbackResponse{
		ObjectKey: objectKey,
		Version:   version,
	}, nil
}
