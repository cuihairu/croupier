// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceConfigDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除 Workspace 配置
func NewWorkspaceConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigDeleteLogic {
	return &WorkspaceConfigDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigDeleteLogic) WorkspaceConfigDelete(req *types.WorkspaceConfigDeleteRequest) (resp *types.WorkspaceConfigDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
