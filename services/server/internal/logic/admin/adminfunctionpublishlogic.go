// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

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

// 发布函数
func NewAdminFunctionPublishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminFunctionPublishLogic {
	return &AdminFunctionPublishLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminFunctionPublishLogic) AdminFunctionPublish(req *types.AdminPublishRequest) (resp *types.AdminFunctionPublishResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
