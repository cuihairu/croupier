
package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type FunctionPermissionsUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新函数权限
func NewFunctionPermissionsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionPermissionsUpdateLogic {
	return &FunctionPermissionsUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionPermissionsUpdateLogic) FunctionPermissionsUpdate(req *FunctionPermissionsUpdateRequest) (*FunctionPermissionsResponse, error) {
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
		return nil, errorx.NewForbidden("无权更新函数权限")
	}

	if l.svcCtx.FunctionModel == nil {
		return nil, errorx.NewInternalError("FunctionModel 未初始化")
	}
	utilsPerms := convertToUtilsPermissions(req.Permissions)
	records, err := utils.ConvertFunctionPermissions(functionID, utilsPerms)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.FunctionModel.ReplacePermissions(l.ctx, functionID, records); err != nil {
		return nil, err
	}

	perms, err := l.svcCtx.FunctionModel.ListPermissions(l.ctx, functionID)
	if err != nil {
		return nil, err
	}

	return &FunctionPermissionsResponse{
		Items: convertFromUtilsPermissions(utils.BuildFunctionPermissions(perms)),
	}, nil
}
