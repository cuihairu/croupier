// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type GameDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除游戏
func NewGameDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameDeleteLogic {
	return &GameDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameDeleteLogic) GameDelete(req *types.GameDeleteRequest) (*types.GameDeleteResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权删除游戏", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.GameModel.Delete(l.ctx, id); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, id)

	return &types.GameDeleteResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id": id,
		},
	}, nil
}
