// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type RoleDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取角色详情
func NewRoleDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDetailLogic {
	return &RoleDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDetailLogic) RoleDetail(req *types.RoleDetailRequest) (*types.RoleDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看角色", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	roleID, err := utils.ParseRoleID(req.ID)
	if err != nil {
		return nil, err
	}

	role, err := l.svcCtx.GetRoleCached(l.ctx, roleID)
	if err != nil {
		return nil, err
	}

	permissions, err := l.svcCtx.GetRolePermissionIDsCached(l.ctx, role.ID)
	if err != nil {
		return nil, err
	}

	return &types.RoleDetailResponse{
		Role: utils.BuildRole(role, permissions),
	}, nil
}
