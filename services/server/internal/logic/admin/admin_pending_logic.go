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

func NewAdminPendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPendingLogic {
	return &AdminPendingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPendingLogic) AdminPending(req *types.AdminPendingRequest) (resp *types.AdminPendingResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
