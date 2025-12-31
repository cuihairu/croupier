// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
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

func (l *GameCreateLogic) GameCreate(req *types.GameCreateRequest) (*types.GameCreateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权创建游戏", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	name, err := sanitizeGameName(req.Name)
	if err != nil {
		return nil, err
	}

	game := &model.Game{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Config:      strings.TrimSpace(req.Config),
		Status:      "dev",
		Enabled:     true,
	}

	if err := l.svcCtx.GameModel.Create(l.ctx, game); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, game.ID)

	return &types.GameCreateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"game": buildGameInfo(game),
		},
	}, nil
}
