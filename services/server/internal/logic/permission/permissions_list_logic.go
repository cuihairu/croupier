// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package permission

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取权限列表
func NewPermissionsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionsListLogic {
	return &PermissionsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionsListLogic) PermissionsList(req *types.PermissionsListRequest) (*types.PermissionsListResponse, error) {
	opts := model.ListPermissionsOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Category: strings.TrimSpace(req.Category),
		Resource: strings.TrimSpace(req.Resource),
	}

	perms, total, err := l.svcCtx.PermissionModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &types.PermissionsListResponse{
		Items: make([]types.Permission, 0, len(perms)),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}

	for i := range perms {
		perm := perms[i]
		resp.Items = append(resp.Items, utils.BuildPermission(&perm))
	}

	return resp, nil
}
