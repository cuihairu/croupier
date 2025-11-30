// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取角色详情
func NewRoleDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDetailLogic {
	return &RoleDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDetailLogic) RoleDetail(req *types.RoleDetailRequest) (*types.RoleDetailResponse, error) {
	roleID, err := utils.ParseRoleID(req.ID)
	if err != nil {
		return nil, err
	}

	role, err := l.svcCtx.RoleModel.FindOne(l.ctx, roleID)
	if err != nil {
		return nil, err
	}

	permissions, err := l.svcCtx.RoleModel.GetRolePermissionIDs(l.ctx, role.ID)
	if err != nil {
		return nil, err
	}

	return &types.RoleDetailResponse{
		Role: utils.BuildRole(role, permissions),
	}, nil
}
