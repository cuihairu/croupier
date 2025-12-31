// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RoleDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除角色
func NewRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleDeleteLogic {
	return &RoleDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleDeleteLogic) RoleDelete(req *types.RoleDeleteRequest) error {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权删除角色", "admin:all", "roles:manage", "role:write"); err != nil {
		return err
	}

	roleID, err := utils.ParseRoleID(req.ID)
	if err != nil {
		return err
	}

	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		roleModel := model.NewRoleModel(tx)
		if _, err := roleModel.FindOne(l.ctx, roleID); err != nil {
			return err
		}

		if err := tx.WithContext(l.ctx).
			Where("role_id = ?", roleID).
			Delete(&model.RolePermission{}).Error; err != nil {
			return fmt.Errorf("删除角色权限失败: %w", err)
		}

		return roleModel.Delete(l.ctx, roleID)
	}); err != nil {
		return err
	}

	l.svcCtx.InvalidateRoleCache(l.ctx, roleID)
	return nil
}
