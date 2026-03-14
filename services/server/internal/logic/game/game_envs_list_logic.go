// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type GameEnvsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取游戏环境列表
func NewGameEnvsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvsListLogic {
	return &GameEnvsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvsListLogic) GameEnvsList(req *types.GameEnvsListRequest) (*types.GameEnvsListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看游戏环境列表", "admin:all", "games:read", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := l.svcCtx.GetGameCached(l.ctx, id)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}

	return &types.GameEnvsListResponse{
		Code:    0,
		Message: "OK",
		Data: types.GameEnvsData{
			Envs: convertGameEnvs(envs),
		},
	}, nil
}
