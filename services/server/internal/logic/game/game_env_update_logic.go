// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新游戏环境
func NewGameEnvUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvUpdateLogic {
	return &GameEnvUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvUpdateLogic) GameEnvUpdate(req *types.GameEnvUpdateRequest) (resp *types.GameEnvUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
