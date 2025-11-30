// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AdminCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建管理员
func NewAdminCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateLogic {
	return &AdminCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateLogic) AdminCreate(req *types.AdminCreateRequest) (*types.AdminCreateResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	password, err := utils.ValidatePassword(req.Password)
	if err != nil {
		return nil, err
	}

	var createdAdmin *model.Admin
	var assignedRoles []model.Role

	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		adminModel := model.NewAdminModel(tx)
		admin := &model.Admin{
			Username: username,
			Nickname: strings.TrimSpace(req.Nickname),
			Email:    strings.TrimSpace(req.Email),
			Phone:    strings.TrimSpace(req.Phone),
			Status:   1,
		}

		if err := adminModel.Create(l.ctx, admin, password); err != nil {
			return err
		}

		if len(req.Roles) > 0 {
			roles, err := fetchRolesByNames(l.ctx, tx, req.Roles)
			if err != nil {
				return err
			}
			for _, role := range roles {
				if err := adminModel.AssignRole(l.ctx, admin.ID, role.ID); err != nil {
					return fmt.Errorf("绑定角色失败: %w", err)
				}
			}
			assignedRoles = roles
		}

		createdAdmin = admin
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.AdminCreateResponse{
		Admin: buildAdminResponse(createdAdmin, roleNamesFromModels(assignedRoles)),
	}, nil
}
