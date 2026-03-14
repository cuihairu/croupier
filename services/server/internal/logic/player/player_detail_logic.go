// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type PlayerDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取玩家详情
func NewPlayerDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerDetailLogic {
	return &PlayerDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerDetailLogic) PlayerDetail(req *types.PlayerDetailRequest) (*types.PlayerDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}

	player, err := l.svcCtx.PlayerModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.PlayerDetailResponse{
		Player: utils.BuildPlayer(player),
	}, nil
}
