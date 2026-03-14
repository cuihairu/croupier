// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"gorm.io/gorm"
)

type WorkspaceConfigDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Workspace 配置
func NewWorkspaceConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigDeleteLogic {
	return &WorkspaceConfigDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigDeleteLogic) WorkspaceConfigDelete(req *types.WorkspaceConfigDeleteRequest) (resp *types.WorkspaceConfigDeleteResponse, err error) {
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
	if err := l.svcCtx.WorkspaceConfigModel.Delete(l.ctx, req.ObjectKey); err != nil {
		return nil, err
	}
	appendWorkspaceAudit(l.ctx, l.svcCtx, "workspace.delete", req.ObjectKey, "success", nil)
	return &types.WorkspaceConfigDeleteResponse{Message: "deleted"}, nil
}
