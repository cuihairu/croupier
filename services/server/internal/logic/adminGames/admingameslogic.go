// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package adminGames

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取管理员的游戏访问权限
func NewAdminGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGamesLogic {
	return &AdminGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGamesLogic) AdminGames(req *types.AdminGamesRequest) (resp *types.AdminGamesResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
