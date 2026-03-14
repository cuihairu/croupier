// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PlayerBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调整玩家余额
func NewPlayerBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerBalanceLogic {
	return &PlayerBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerBalanceLogic) PlayerBalance(req *types.PlayerBalanceRequest) (*types.PlayerBalanceResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("调整原因不能为空")
	}
	player, err := l.svcCtx.PlayerModel.UpdateBalance(l.ctx, id, req.Amount, reason)
	if err != nil {
		return nil, err
	}

	return &types.PlayerBalanceResponse{
		Player: utils.BuildPlayer(player),
	}, nil
}
