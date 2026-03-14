
package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionPermissionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数权限
func NewFunctionPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionPermissionsLogic {
	return &FunctionPermissionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionPermissionsLogic) FunctionPermissions(req *FunctionPermissionsRequest) (*FunctionPermissionsResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "permission:write") && !utils.HasPermissionID(permIDs, "roles:manage") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看函数权限")
	}

	if l.svcCtx.FunctionModel == nil {
		return nil, errorx.NewInternalError("FunctionModel 未初始化")
	}
	perms, err := l.svcCtx.FunctionModel.ListPermissions(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	return &FunctionPermissionsResponse{
		Items: convertFromUtilsPermissions(utils.BuildFunctionPermissions(perms)),
	}, nil
}
