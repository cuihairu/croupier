// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type GameEnvUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新游戏环境
func NewGameEnvUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvUpdateLogic {
	return &GameEnvUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvUpdateLogic) GameEnvUpdate(req *types.GameEnvUpdateRequest) (*types.GameEnvUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权更新游戏环境", "admin:all", "games:manage"); err != nil {
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

	target := envs[idx]
	if newName := strings.TrimSpace(req.Name); newName != "" {
		if other := findEnvIndex(envs, newName); other >= 0 && other != idx {
			return nil, errorx.NewConflict("环境 " + newName + " 已存在")
		}
		target.Env = newName
	}
	if v := strings.TrimSpace(req.Type); v != "" {
		target.Description = v
	}
	envs[idx] = target

	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}
	if err := l.svcCtx.GameModel.Update(l.ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, id)

	return &types.GameEnvUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"envs": convertGameEnvs(envs),
		},
	}, nil
}
