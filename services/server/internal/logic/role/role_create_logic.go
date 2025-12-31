// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RoleCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建角色
func NewRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleCreateLogic {
	return &RoleCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleCreateLogic) RoleCreate(req *types.RoleCreateRequest) (*types.RoleCreateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权创建角色", "admin:all", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("角色名称不能为空")
	}

	permissionIDs, err := utils.EnsurePermissionIDs(l.ctx, l.svcCtx.RoleModel, req.Permissions)
	if err != nil {
		return nil, err
	}

	var createdRole *model.Role
	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		role := &model.Role{
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			Category:    strings.TrimSpace(req.Category),
		}
		roleModel := model.NewRoleModel(tx)
		if err := roleModel.Create(l.ctx, role); err != nil {
			return err
		}
		if err := roleModel.ReplacePermissions(l.ctx, role.ID, permissionIDs); err != nil {
			return err
		}
		createdRole = role
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.RoleCreateResponse{
		Role: utils.BuildRole(createdRole, permissionIDs),
	}, nil
}
