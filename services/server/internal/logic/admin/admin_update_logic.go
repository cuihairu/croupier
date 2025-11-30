// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AdminUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新管理员
func NewAdminUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateLogic {
	return &AdminUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateLogic) AdminUpdate(req *types.AdminUpdateRequest) (*types.AdminUpdateResponse, error) {
	adminID, err := parseAdminID(req.ID)
	if err != nil {
		return nil, err
	}

	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)

		if _, err := adminModel.FindOne(l.ctx, adminID); err != nil {
			return err
		}

		updates := make(map[string]interface{})
		if nickname := strings.TrimSpace(req.Nickname); nickname != "" {
			updates["nickname"] = nickname
		}
		if email := strings.TrimSpace(req.Email); email != "" {
			updates["email"] = email
		}
		if phone := strings.TrimSpace(req.Phone); phone != "" {
			updates["phone"] = phone
		}
		if req.Status != -1 {
			updates["status"] = req.Status
		}

		if len(updates) > 0 {
			if err := adminModel.Update(l.ctx, adminID, updates); err != nil {
				return err
			}
		}

		if req.Roles != nil {
			if err := tx.WithContext(l.ctx).
				Where("admin_id = ?", adminID).
				Delete(&model.AdminRole{}).Error; err != nil {
				return fmt.Errorf("清理旧角色失败: %w", err)
			}

			if len(req.Roles) > 0 {
				roles, err := fetchRolesByNames(l.ctx, tx, req.Roles)
				if err != nil {
					return err
				}
				for _, role := range roles {
					if err := adminModel.AssignRole(l.ctx, adminID, role.ID); err != nil {
						return fmt.Errorf("分配角色失败: %w", err)
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	admin, err := l.svcCtx.AdminModel.FindOne(l.ctx, adminID)
	if err != nil {
		return nil, err
	}

	roles, err := l.svcCtx.AdminModel.GetAdminRoles(l.ctx, adminID)
	if err != nil {
		return nil, fmt.Errorf("获取管理员角色失败: %w", err)
	}

	return &types.AdminUpdateResponse{
		Admin: buildAdminResponse(admin, roleNamesFromModels(roles)),
	}, nil
}
