package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WorkspaceRollbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceRollbackLogic {
	return &WorkspaceRollbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceRollbackLogic) Rollback(objectKey, versionID string) (map[string]interface{}, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	versionID = strings.TrimSpace(versionID)
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

	cfg, err := parseWorkspaceDTOFromVersion(record.Value)
	if err != nil {
		return nil, errorx.NewBadRequest("workspace version payload is invalid")
	}
	cfg.ObjectKey = objectKey
	cfg.Status = resolveWorkspaceStatus(cfg)

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, errorx.NewInternalError("failed to encode workspace config")
	}

	modelRecord := &model.WorkspaceConfig{
		ObjectKey:   objectKey,
		Title:       cfg.Title,
		Published:   cfg.Published,
		PublishedAt: parseTimePtr(cfg.PublishedAt),
		PublishedBy: cfg.PublishedBy,
		MenuOrder:   cfg.MenuOrder,
		Config:      configJSON,
	}
	if err := l.svcCtx.WorkspaceConfigModel.Upsert(l.ctx, modelRecord); err != nil {
		return nil, err
	}

	actor := workspaceActorFromCtx(l.ctx)
	_, _ = persistWorkspaceVersion(l.ctx, l.svcCtx, *cfg, actor, fmt.Sprintf("rollback to v%d", version))
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.rollback", objectKey, "success", map[string]interface{}{
		"version": version,
	})

	return map[string]interface{}{
		"objectKey": objectKey,
		"version":   version,
	}, nil
}
