// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceConfigSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 保存 Workspace 配置（创建或更新）
func NewWorkspaceConfigSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigSaveLogic {
	return &WorkspaceConfigSaveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigSaveLogic) WorkspaceConfigSave(req *types.WorkspaceConfigSaveRequest) (resp *types.WorkspaceConfigSaveResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
