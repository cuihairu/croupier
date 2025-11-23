package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionPermissionsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionPermissionsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPermissionsUpdateLogic {
	return &AdminFunctionPermissionsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPermissionsUpdateLogic) AdminFunctionPermissionsUpdate(req *types.AdminFunctionPermissionsUpdateRequest) (resp *types.AdminFunctionPermissionsUpdateResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
