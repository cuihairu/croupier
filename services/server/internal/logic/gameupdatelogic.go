package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameUpdateLogic {
	return &GameUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameUpdateLogic) GameUpdate(req *types.GameUpdateRequest) error {
	if req == nil {
		return ErrInvalidRequest
	}
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return svc.ErrGameNotFound
	}
	game, err := repo.Get(l.ctx, uint(req.Id))
	if err != nil {
		return err
	}
	applyGameUpdates(game, req)
	return repo.Update(l.ctx, game)
}

func applyGameUpdates(game *ports.Game, req *types.GameUpdateRequest) {
	if strings.TrimSpace(req.Name) != "" {
		game.Name = strings.TrimSpace(req.Name)
	}
	game.Icon = strings.TrimSpace(req.Icon)
	game.Description = strings.TrimSpace(req.Description)
	game.Enabled = req.Enabled
	if strings.TrimSpace(req.Status) != "" {
		game.Status = normalizeGameStatus(req.Status)
	}
	game.AliasName = strings.TrimSpace(req.AliasName)
	game.Homepage = strings.TrimSpace(req.Homepage)
	if strings.TrimSpace(req.GameType) != "" {
		game.GameType = strings.TrimSpace(req.GameType)
	}
	if strings.TrimSpace(req.GenreCode) != "" {
		game.GenreCode = strings.TrimSpace(req.GenreCode)
	}
}
