// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package adminGames

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type AdminGamesUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新管理员的游戏访问权限
func NewAdminGamesUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGamesUpdateLogic {
	return &AdminGamesUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGamesUpdateLogic) AdminGamesUpdate(req *types.AdminGamesUpdateRequest) error {
	return errorx.NewNotImplemented("AdminGamesUpdate not implemented")
}
