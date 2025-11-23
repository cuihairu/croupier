// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RolesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取角色列表
func NewRolesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolesListLogic {
	return &RolesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RolesListLogic) RolesList(req *types.RolesListRequest) (resp *types.RolesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
