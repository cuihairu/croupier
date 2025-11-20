package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameUpsertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameUpsertLogic {
	return &GameUpsertLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameUpsertLogic) GameUpsert(req *types.GameUpsertRequest) (*types.GameUpsertResponse, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" {
		return nil, ErrInvalidRequest
	}
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return nil, fmt.Errorf("games repository unavailable")
	}
	if req.Id <= 0 {
		game := &ports.Game{
			Name:        strings.TrimSpace(req.Name),
			Icon:        strings.TrimSpace(req.Icon),
			Description: strings.TrimSpace(req.Description),
			Enabled:     req.Enabled,
			Status:      normalizeGameStatus(req.Status),
			AliasName:   strings.TrimSpace(req.AliasName),
			Homepage:    strings.TrimSpace(req.Homepage),
			GameType:    strings.TrimSpace(req.GameType),
			GenreCode:   strings.TrimSpace(req.GenreCode),
		}
		if err := repo.Create(l.ctx, game); err != nil {
			return nil, err
		}
		return &types.GameUpsertResponse{Id: int64(game.ID)}, nil
	}
	// update path
	game, err := repo.Get(l.ctx, uint(req.Id))
	if err != nil {
		if errors.Is(err, svc.ErrGameNotFound) {
			return nil, ErrInvalidRequest
		}
		return nil, err
	}
	game.Name = strings.TrimSpace(req.Name)
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
	if err := repo.Update(l.ctx, game); err != nil {
		return nil, err
	}
	return &types.GameUpsertResponse{Id: int64(game.ID)}, nil
}
