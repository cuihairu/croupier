// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayersListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取玩家列表
func NewPlayersListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayersListLogic {
	return &PlayersListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayersListLogic) PlayersList(req *types.PlayersListRequest) (*types.PlayersListResponse, error) {
	opts := model.ListPlayersOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		GameID:   strings.TrimSpace(req.GameId),
		Search:   strings.TrimSpace(req.Search),
	}
	if req.Status != 0 {
		status := req.Status
		opts.Status = &status
	}
	if req.Level != 0 {
		level := req.Level
		opts.Level = &level
	}
	if req.Vip != 0 {
		vip := req.Vip
		opts.VIP = &vip
	}

	players, total, err := l.svcCtx.PlayerModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]types.Player, 0, len(players))
	for i := range players {
		items = append(items, utils.BuildPlayer(&players[i]))
	}

	return &types.PlayersListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
