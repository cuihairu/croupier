// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除游戏
func NewGameDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameDeleteLogic {
	return &GameDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameDeleteLogic) GameDelete(req *types.GameDeleteRequest) (resp *types.GameDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
