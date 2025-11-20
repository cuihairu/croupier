package logic

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GamesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGamesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GamesListLogic {
	return &GamesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GamesListLogic) GamesList() (*types.GamesListResponse, error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return &types.GamesListResponse{Games: []types.GameInfo{}}, nil
	}
	items, err := repo.List(l.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.GameInfo, 0, len(items))
	for _, g := range items {
		var envs []*ports.GameEnvDef
		if repo != nil {
			envs, err = repo.ListEnvRecords(l.ctx, g.ID)
			if err != nil && !errors.Is(err, svc.ErrGameNotFound) {
				return nil, err
			}
		}
		out = append(out, gameToInfo(g, envs))
	}
	return &types.GamesListResponse{Games: out}, nil
}
