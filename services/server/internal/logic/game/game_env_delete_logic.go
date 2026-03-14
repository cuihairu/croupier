// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type GameEnvDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除游戏环境
func NewGameEnvDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvDeleteLogic {
	return &GameEnvDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvDeleteLogic) GameEnvDelete(req *types.GameEnvDeleteRequest) (*types.GameEnvDeleteResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权删除游戏环境", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := l.svcCtx.GameModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}

	idx := findEnvIndex(envs, req.EnvID)
	if idx < 0 {
		return nil, errorx.NewNotFound("环境 " + req.EnvID + " 不存在")
	}

	envs = append(envs[:idx], envs[idx+1:]...)
	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}
	if err := l.svcCtx.GameModel.Update(l.ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, id)

	return &types.GameEnvDeleteResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"envs": convertGameEnvs(envs),
		},
	}, nil
}
