// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspacePublishedListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取已发布的 Workspace 配置列表
func NewWorkspacePublishedListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspacePublishedListLogic {
	return &WorkspacePublishedListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspacePublishedListLogic) WorkspacePublishedList(req *types.WorkspacePublishedListRequest) (resp *types.WorkspacePublishedListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
