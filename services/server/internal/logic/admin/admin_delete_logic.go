// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"gorm.io/gorm"
)

type AdminDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除管理员
func NewAdminDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteLogic {
	return &AdminDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteLogic) AdminDelete(req *types.AdminDeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权删除管理员", "admin:all", "user:write"); err != nil {
		return err
	}

	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return err
	}

	var existing *model.Admin
	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)

		adminRecord, err := adminModel.FindOne(l.ctx, adminID)
		if err != nil {
			return err
		}
		existing = adminRecord

		if err := tx.WithContext(l.ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminRole{}).Error; err != nil {
			return errorx.NewInternalError("删除角色绑定失败")
		}

		if err := tx.WithContext(l.ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameScope{}).Error; err != nil {
			return errorx.NewInternalError("删除游戏范围失败")
		}

		if err := tx.WithContext(l.ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameEnvScope{}).Error; err != nil {
			return errorx.NewInternalError("删除环境范围失败")
		}

		return adminModel.Delete(l.ctx, adminID)
	}); err != nil {
		return err
	}

	username := ""
	if existing != nil {
		username = existing.Username
	}
	l.svcCtx.InvalidateAdminCache(l.ctx, adminID, username)

	return nil
}
