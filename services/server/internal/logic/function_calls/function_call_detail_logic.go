package function_calls

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionCallDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionCallDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCallDetailLogic {
	return &FunctionCallDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCallDetailLogic) FunctionCallDetail(req *types.FunctionCallDetailRequest) (*types.FunctionCallDetailResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "function_calls:read") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看调用详情")
	}

	// Parse ID
	id, err := parseID(req.ID)
	if err != nil {
		return nil, errorx.NewBadRequest("无效的记录ID")
	}

	// Query database
	history, err := l.svcCtx.JobHistoryModel.FindByID(l.ctx, id)
	if err != nil {
		return nil, errorx.NewNotFound("调用记录不存在")
	}

	item := convertToFunctionCallItem(history)
	return &types.FunctionCallDetailResponse{FunctionCallItem: item}, nil
}

func (l *FunctionCallDetailLogic) FunctionCallDetailByJobID(jobID string) (*types.FunctionCallDetailResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "function_calls:read") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权查看调用详情")
	}

	// Query by job ID
	history, err := l.svcCtx.JobHistoryModel.FindByJobID(l.ctx, jobID)
	if err != nil {
		return nil, errorx.NewNotFound("调用记录不存在")
	}

	item := convertToFunctionCallItem(history)
	return &types.FunctionCallDetailResponse{FunctionCallItem: item}, nil
}

// GetCreatedAt converts time.Time to RFC3339 string
func GetCreatedAt(t time.Time) string {
	return t.Format(time.RFC3339)
}
