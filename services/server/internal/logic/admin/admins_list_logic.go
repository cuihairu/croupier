// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package admin

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取管理员列表
func NewAdminsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminsListLogic {
	return &AdminsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminsListLogic) AdminsList(req *types.AdminsListRequest) (resp *types.AdminsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
