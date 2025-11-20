package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameDetailLogic {
	return &GameDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameDetailLogic) GameDetail(req *types.GameDetailRequest) (*types.GameDetailResponse, error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return nil, svc.ErrGameNotFound
	}
	game, err := repo.Get(l.ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	var envs []*ports.GameEnvDef
	if envs, err = repo.ListEnvRecords(l.ctx, uint(req.Id)); err != nil && !errors.Is(err, svc.ErrGameNotFound) {
		return nil, err
	}
	info := gameToInfo(game, envs)
	return &types.GameDetailResponse{Game: info}, nil
}
