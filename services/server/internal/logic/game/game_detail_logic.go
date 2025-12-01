// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取游戏详情
func NewGameDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameDetailLogic {
	return &GameDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameDetailLogic) GameDetail(req *types.GameDetailRequest) (*types.GameDetailResponse, error) {
	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := l.svcCtx.GetGameCached(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.GameDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    buildGameInfo(game),
	}, nil
}
