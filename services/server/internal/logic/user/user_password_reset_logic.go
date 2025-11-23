// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserPasswordResetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重置用户密码
func NewUserPasswordResetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPasswordResetLogic {
	return &UserPasswordResetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserPasswordResetLogic) UserPasswordReset(req *types.UserPasswordResetRequest) (resp *types.UserPasswordResetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
