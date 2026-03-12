// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayerBalanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调整玩家余额
func NewPlayerBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerBalanceLogic {
	return &PlayerBalanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerBalanceLogic) PlayerBalance(req *types.PlayerBalanceRequest) (resp *types.PlayerBalanceResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
