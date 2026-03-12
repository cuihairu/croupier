// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package permission

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取权限列表
func NewPermissionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionsListLogic {
	return &PermissionsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionsListLogic) PermissionsList(req *types.PermissionsListRequest) (resp *types.PermissionsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
