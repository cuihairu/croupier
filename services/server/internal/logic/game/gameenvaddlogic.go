// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加游戏环境
func NewGameEnvAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvAddLogic {
	return &GameEnvAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvAddLogic) GameEnvAdd(req *types.GameEnvAddRequest) (resp *types.GameEnvAddResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
