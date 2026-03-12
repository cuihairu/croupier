// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceVersionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Workspace 单个版本详情
func NewWorkspaceVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceVersionDetailLogic {
	return &WorkspaceVersionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceVersionDetailLogic) WorkspaceVersionDetail(req *types.WorkspaceVersionDetailRequest) (resp *types.WorkspaceVersionDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
