// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package user

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取用户游戏
func NewUserGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGamesLogic {
	return &UserGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserGamesLogic) UserGames(req *types.UserGamesRequest) (resp *types.UserGamesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
