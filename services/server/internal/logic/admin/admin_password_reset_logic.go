// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type AdminPasswordResetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重置管理员密码
func NewAdminPasswordResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPasswordResetLogic {
	return &AdminPasswordResetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPasswordResetLogic) AdminPasswordReset(req *types.AdminPasswordResetRequest) error {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权重置管理员密码", "admin:all", "user:write"); err != nil {
		return err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return err
	}

	password, err := utils.ValidatePassword(req.NewPassword)
	if err != nil {
		return err
	}

	admin, err := l.svcCtx.AdminModel.FindOne(l.ctx, adminID)
	if err != nil {
		return err
	}

	if err := l.svcCtx.AdminModel.UpdatePassword(l.ctx, adminID, password); err != nil {
		return err
	}

	l.svcCtx.InvalidateAdminCache(l.ctx, adminID, admin.Username)

	return nil
}
