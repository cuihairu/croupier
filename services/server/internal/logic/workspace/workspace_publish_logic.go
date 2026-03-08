// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WorkspacePublishLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发布 Workspace 配置
func NewWorkspacePublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspacePublishLogic {
	return &WorkspacePublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspacePublishLogic) WorkspacePublish(req *types.WorkspacePublishRequest) (resp *types.WorkspacePublishResponse, err error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	current, err := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
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
	if err := l.svcCtx.WorkspaceConfigModel.SetPublished(l.ctx, req.ObjectKey, true, req.PublishedBy); err != nil {
		return nil, err
	}
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.publish", req.ObjectKey, "success", map[string]interface{}{
		"publishedBy": strings.TrimSpace(req.PublishedBy),
	})
	if current, findErr := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey); findErr == nil {
		dto := toDTO(current)
		actor := strings.TrimSpace(req.PublishedBy)
		if actor == "" {
			actor = workspaceActorFromCtx(l.ctx)
		}
		_, _ = persistWorkspaceVersion(l.ctx, l.svcCtx, dto, actor, "publish workspace config")
	}
	return &types.WorkspacePublishResponse{Published: true, ObjectKey: req.ObjectKey}, nil
}
