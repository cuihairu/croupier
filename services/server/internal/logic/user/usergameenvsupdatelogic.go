// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserGameEnvsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新用户游戏环境
func NewUserGameEnvsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGameEnvsUpdateLogic {
	return &UserGameEnvsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserGameEnvsUpdateLogic) UserGameEnvsUpdate(req *types.UserGameEnvsUpdateRequest) (resp *types.UserGameEnvsUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
