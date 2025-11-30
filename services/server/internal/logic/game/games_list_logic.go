// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
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

func (l *GamesListLogic) GamesList(req *types.GamesListRequest) (*types.GamesListResponse, error) {
	opts := model.ListGamesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   strings.TrimSpace(req.Status),
	}

	games, total, err := l.svcCtx.GameModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.GameInfo, 0, len(games))
	for i := range games {
		items = append(items, buildGameInfo(&games[i]))
	}

	return &types.GamesListResponse{
		Code:    0,
		Message: "OK",
		Data: types.GamesData{
			Games: items,
			Total: int(total),
		},
	}, nil
}
