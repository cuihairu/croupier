// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type WorkspaceConfigsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取所有 Workspace 配置列表
func NewWorkspaceConfigsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceConfigsListLogic {
	return &WorkspaceConfigsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceConfigsListLogic) WorkspaceConfigsList(req *types.WorkspaceConfigsListRequest) (resp *types.WorkspaceConfigsListResponse, err error) {
	items, err := l.svcCtx.WorkspaceConfigModel.ListAll(l.ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]types.WorkspaceConfig, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i])
		_ = enrichWorkspaceVersion(l.ctx, l.svcCtx, &dto)
		dtos = append(dtos, dto)
	}
	return &types.WorkspaceConfigsListResponse{Items: dtos}, nil
}
