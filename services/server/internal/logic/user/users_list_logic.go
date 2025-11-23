// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UsersListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取用户列表
func NewUsersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UsersListLogic {
	return &UsersListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UsersListLogic) UsersList(req *types.UsersListRequest) (resp *types.UsersListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
