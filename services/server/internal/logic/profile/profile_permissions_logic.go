// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"

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
	// todo: add your logic here and delete this line

	return
}
