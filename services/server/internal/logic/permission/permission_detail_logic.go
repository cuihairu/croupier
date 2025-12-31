// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package permission

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取权限详情
func NewPermissionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionDetailLogic {
	return &PermissionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionDetailLogic) PermissionDetail(req *types.PermissionDetailRequest) (*types.PermissionDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看权限详情", "admin:all", "permission:read", "permission:write"); err != nil {
		return nil, err
	}

	id, err := utils.ValidatePermissionID(req.ID)
	if err != nil {
		return nil, err
	}

	perm, err := l.svcCtx.GetPermissionCached(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.PermissionDetailResponse{
		Permission: utils.BuildPermission(perm),
	}, nil
}
