// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfilePermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取当前用户权限
func NewProfilePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfilePermissionsLogic {
	return &ProfilePermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfilePermissionsLogic) ProfilePermissions(req *types.ProfilePermissionsRequest) (resp *types.ProfilePermissionsResponse, err error) {
	admin, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	if l.svcCtx.ProfileModel == nil {
		return nil, errors.New("ProfileModel 未初始化")
	}

	perms, err := l.svcCtx.ProfileModel.ListPermissions(l.ctx, admin.ID)
	if err != nil {
		return nil, fmt.Errorf("获取权限列表失败: %w", err)
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	respPerms := make([]types.ProfilePermission, 0, len(perms))
	for i := range perms {
		record := perms[i]
		if gameID != "" && !strings.EqualFold(record.GameID, gameID) {
			continue
		}
		if env != "" && !strings.EqualFold(record.Env, env) {
			continue
		}
		respPerms = append(respPerms, types.ProfilePermission{
			Resource: record.Resource,
			Actions:  utils.DecodeStringSlice(record.Actions),
			GameId:   record.GameID,
			Env:      record.Env,
		})
	}

	roleNames := utils.RoleNamesFromModels(roles)
	return &types.ProfilePermissionsResponse{
		Permissions: respPerms,
		Admin:       utils.HasAdminRole(roleNames),
		Roles:       roleNames,
	}, nil
}
