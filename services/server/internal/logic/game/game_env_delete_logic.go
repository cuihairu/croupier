// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除游戏环境
func NewGameEnvDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvDeleteLogic {
	return &GameEnvDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvDeleteLogic) GameEnvDelete(req *types.GameEnvDeleteRequest) (resp *types.GameEnvDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
