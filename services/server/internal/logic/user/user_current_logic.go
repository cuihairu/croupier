// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserCurrentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取当前用户
func NewUserCurrentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserCurrentLogic {
	return &UserCurrentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserCurrentLogic) UserCurrent(req *types.UserCurrentRequest) (resp *types.UserCurrentResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
