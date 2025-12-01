// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AdminDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除管理员
func NewAdminDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteLogic {
	return &AdminDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteLogic) AdminDelete(req *types.AdminDeleteRequest) error {
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
			return fmt.Errorf("删除角色绑定失败: %w", err)
		}

		if err := tx.WithContext(l.ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameScope{}).Error; err != nil {
			return fmt.Errorf("删除游戏范围失败: %w", err)
		}

		if err := tx.WithContext(l.ctx).
			Where("admin_id = ?", adminID).
			Delete(&model.AdminGameEnvScope{}).Error; err != nil {
			return fmt.Errorf("删除环境范围失败: %w", err)
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
