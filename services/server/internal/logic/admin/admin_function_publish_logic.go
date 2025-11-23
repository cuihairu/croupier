package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminFunctionPublishLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminFunctionPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPublishLogic {
	return &AdminFunctionPublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPublishLogic) AdminFunctionPublish(req *types.AdminPublishRequest) (resp *types.AdminFunctionPublishResponse, err error) {
	// TODO: add your logic here and delete this line

	return
}
