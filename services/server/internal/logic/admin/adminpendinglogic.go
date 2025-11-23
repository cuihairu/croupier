// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminPendingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取待处理项
func NewAdminPendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPendingLogic {
	return &AdminPendingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPendingLogic) AdminPending(req *types.AdminPendingRequest) (resp *types.AdminPendingResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
