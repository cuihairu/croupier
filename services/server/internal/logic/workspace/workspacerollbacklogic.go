// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceRollbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 回滚 Workspace 到指定版本
func NewWorkspaceRollbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceRollbackLogic {
	return &WorkspaceRollbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceRollbackLogic) WorkspaceRollback(req *types.WorkspaceRollbackRequest) (resp *types.WorkspaceRollbackResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
