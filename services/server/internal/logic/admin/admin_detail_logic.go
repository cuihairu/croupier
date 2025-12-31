// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取管理员详情
func NewAdminDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDetailLogic {
	return &AdminDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDetailLogic) AdminDetail(req *types.AdminDetailRequest) (*types.AdminDetailResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看管理员", "admin:all", "user:read", "user:write"); err != nil {
		return nil, err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	admin, err := l.svcCtx.GetAdminCached(l.ctx, adminID)
	if err != nil {
		return nil, err
	}

	roles, err := l.svcCtx.GetAdminRolesCached(l.ctx, admin.ID)
	if err != nil {
		return nil, fmt.Errorf("获取管理员角色失败: %w", err)
	}

	return &types.AdminDetailResponse{
		Admin: buildAdminResponse(admin, roleNamesFromModels(roles)),
	}, nil
}
