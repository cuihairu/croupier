// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package me

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MeGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取我的游戏
func NewMeGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeGamesLogic {
	return &MeGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MeGamesLogic) MeGames(req *types.MeGamesRequest) (resp *types.MeGamesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
