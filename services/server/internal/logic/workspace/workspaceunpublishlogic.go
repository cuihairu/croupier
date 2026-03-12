// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return
}
