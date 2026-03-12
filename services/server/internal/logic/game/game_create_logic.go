// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建游戏
func NewGameCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameCreateLogic {
	return &GameCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameCreateLogic) GameCreate(req *types.GameCreateRequest) (resp *types.GameCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
