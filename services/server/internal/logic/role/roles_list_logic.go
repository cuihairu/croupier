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
)

type RolesListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取角色列表
func NewRolesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RolesListLogic {
	return &RolesListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RolesListLogic) RolesList(req *types.RolesListRequest) (*types.RolesListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看角色列表", "admin:all", "roles:read", "role:read", "roles:manage", "role:write"); err != nil {
		return nil, err
	}

	opts := model.ListRolesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Category: strings.TrimSpace(req.Category),
		Search:   strings.TrimSpace(req.Search),
	}

	roles, total, err := l.svcCtx.RoleModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &types.RolesListResponse{
		Items: make([]types.Role, 0, len(roles)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}

	if len(roles) == 0 {
		return resp, nil
	}

	roleIDs := make([]uint, 0, len(roles))
	for _, role := range roles {
		roleIDs = append(roleIDs, role.ID)
	}

	permMap, err := l.svcCtx.RoleModel.GetRolesPermissionIDs(l.ctx, roleIDs)
	// continue even if permMap empty? but check err.
	if err != nil {
		return nil, err
	}

	for i := range roles {
		role := roles[i]
		resp.Items = append(resp.Items, utils.BuildRole(&role, permMap[role.ID]))
	}

	return resp, nil
}
