// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GamesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取游戏列表
func NewGamesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GamesListLogic {
	return &GamesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GamesListLogic) GamesList(req *types.GamesListRequest) (resp *types.GamesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
