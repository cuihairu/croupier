// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建管理员
func NewAdminCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateLogic {
	return &AdminCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateLogic) AdminCreate(req *types.AdminCreateRequest) (resp *types.AdminCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
