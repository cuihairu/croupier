// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WorkspaceUnpublishLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 取消发布 Workspace 配置
func NewWorkspaceUnpublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceUnpublishLogic {
	return &WorkspaceUnpublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceUnpublishLogic) WorkspaceUnpublish(req *types.WorkspaceUnpublishRequest) (resp *types.WorkspaceUnpublishResponse, err error) {
	if req.ObjectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	_, err = l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace config not found")
		}
		return nil, err
	}
	if err := l.svcCtx.WorkspaceConfigModel.SetPublished(l.ctx, req.ObjectKey, false, ""); err != nil {
		return nil, err
	}
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.unpublish", req.ObjectKey, "success", nil)
	if current, findErr := l.svcCtx.WorkspaceConfigModel.FindByObjectKey(l.ctx, req.ObjectKey); findErr == nil {
		dto := toDTO(current)
		actor := workspaceActorFromCtx(l.ctx)
		_, _ = persistWorkspaceVersion(l.ctx, l.svcCtx, dto, actor, "unpublish workspace config")
	}
	return &types.WorkspaceUnpublishResponse{Published: false, ObjectKey: req.ObjectKey}, nil
}
