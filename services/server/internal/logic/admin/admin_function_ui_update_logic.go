package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionUIUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionUIUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionUIUpdateLogic {
	return &AdminFunctionUIUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionUIUpdateLogic) AdminFunctionUIUpdate(req *types.AdminFunctionUIUpdateRequest) (resp *types.AdminFunctionUIUpdateResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
