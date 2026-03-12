// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"

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

func (l *PlayersListLogic) PlayersList(req *types.PlayersListRequest) (resp *types.PlayersListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
