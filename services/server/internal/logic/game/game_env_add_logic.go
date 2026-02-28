// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加游戏环境
func NewGameEnvAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvAddLogic {
	return &GameEnvAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvAddLogic) GameEnvAdd(req *types.GameEnvAddRequest) (*types.GameEnvAddResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权添加游戏环境", "admin:all", "games:manage"); err != nil {
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

	newEnv, err := ensureEnvName(req.Name)
	if err != nil {
		return nil, err
	}

	envs, err := game.GetEnvs()
	if err != nil {
		return nil, err
	}
	if findEnvIndex(envs, newEnv) >= 0 {
		return nil, errorx.NewConflict("环境 " + newEnv + " 已存在")
	}

	envs = append(envs, model.GameEnv{
		Env:         newEnv,
		Description: strings.TrimSpace(req.Type),
	})
	if err := game.SetEnvs(envs); err != nil {
		return nil, err
	}

	if err := l.svcCtx.GameModel.Update(l.ctx, id, map[string]interface{}{"envs": game.Envs}); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, id)

	return &types.GameEnvAddResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"envs": convertGameEnvs(envs),
		},
	}, nil
}
