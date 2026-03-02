// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
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
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	if l.svcCtx.PermissionModel == nil || l.svcCtx.RoleModel == nil {
		return nil, errors.New("PermissionModel/RoleModel 未初始化")
	}

	permissionIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, errorx.NewInternalError("获取权限列表失败")
	}

	gameID := strings.TrimSpace(req.GameId)
	env := strings.TrimSpace(req.Env)
	_ = gameID
	_ = env

	// Build resource->actions view for convenience (derived from permission records).
	byRes := make(map[string]map[string]struct{})
	for _, pid := range permissionIDs {
		perm, err := l.svcCtx.GetPermissionCached(l.ctx, pid)
		if err != nil || perm == nil {
			continue
		}
		res := strings.TrimSpace(perm.Resource)
		act := strings.TrimSpace(perm.Action)
		if res == "" {
			res = "global"
		}
		if act == "" {
			act = "*"
		}
		actSet := byRes[res]
		if actSet == nil {
			actSet = map[string]struct{}{}
			byRes[res] = actSet
		}
		actSet[act] = struct{}{}
	}

	respPerms := make([]types.ProfilePermission, 0, len(byRes))
	for res, acts := range byRes {
		list := make([]string, 0, len(acts))
		for a := range acts {
			list = append(list, a)
		}
		respPerms = append(respPerms, types.ProfilePermission{
			Resource: res,
			Actions:  list,
		})
	}

	roleNames := utils.RoleNamesFromModels(roles)
	isAdmin := utils.HasAdminRole(roleNames)

	// For UIs that still rely on "*" semantics.
	if isAdmin && !utils.HasPermissionID(permissionIDs, "*") {
		permissionIDs = append(permissionIDs, "*")
	}
	return &types.ProfilePermissionsResponse{
		Permissions:   respPerms,
		Admin:         isAdmin,
		Roles:         roleNames,
		PermissionIDs: permissionIDs,
	}, nil
}
