// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package permission

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PermissionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取权限详情
func NewPermissionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PermissionDetailLogic {
	return &PermissionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PermissionDetailLogic) PermissionDetail(req *types.PermissionDetailRequest) (resp *types.PermissionDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
