package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionUIGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionUIGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionUIGetLogic {
	return &AdminFunctionUIGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionUIGetLogic) AdminFunctionUIGet(req *types.AdminFunctionRequest) (resp *types.AdminFunctionUIGetResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
