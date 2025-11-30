// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package role

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RoleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新角色
func NewRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleUpdateLogic {
	return &RoleUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleUpdateLogic) RoleUpdate(req *types.RoleUpdateRequest) (*types.RoleUpdateResponse, error) {
	roleID, err := utils.ParseRoleID(req.ID)
	if err != nil {
		return nil, err
	}

	permissionIDs := req.Permissions
	updatePermissions := permissionIDs != nil
	var normalizedPermissions []string
	if updatePermissions {
		normalizedPermissions, err = utils.EnsurePermissionIDs(l.ctx, l.svcCtx.RoleModel, permissionIDs)
		if err != nil {
			return nil, err
		}
	}

	err = l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		roleModel := model.NewRoleModel(tx)
		if _, err := roleModel.FindOne(l.ctx, roleID); err != nil {
			return err
		}

		updates := make(map[string]interface{})
		if name := strings.TrimSpace(req.Name); name != "" {
			updates["name"] = name
		}
		if desc := strings.TrimSpace(req.Description); desc != "" {
			updates["description"] = desc
		}
		if category := strings.TrimSpace(req.Category); category != "" {
			updates["category"] = category
		}

		if err := roleModel.Update(l.ctx, roleID, updates); err != nil {
			return err
		}

		if updatePermissions {
			if err := roleModel.ReplacePermissions(l.ctx, roleID, normalizedPermissions); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	role, err := l.svcCtx.RoleModel.FindOne(l.ctx, roleID)
	if err != nil {
		return nil, err
	}

	perms := normalizedPermissions
	if !updatePermissions {
		perms, err = l.svcCtx.RoleModel.GetRolePermissionIDs(l.ctx, roleID)
		if err != nil {
			return nil, err
		}
	}

	return &types.RoleUpdateResponse{
		Role: utils.BuildRole(role, perms),
	}, nil
}
