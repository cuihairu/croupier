// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取游戏环境列表
func NewGameEnvsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvsListLogic {
	return &GameEnvsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvsListLogic) GameEnvsList(req *types.GameEnvsListRequest) (resp *types.GameEnvsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
