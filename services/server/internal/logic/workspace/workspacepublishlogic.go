// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return
}
