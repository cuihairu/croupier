package game

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

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

func (l *GameDetailLogic) GameDetail(req *types.GameDetailRequest) (resp *types.GameDetailResponse, err error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return nil, svc.ErrGameNotFound
	}

	gameID, err := parseID(req.ID)
	if err != nil {
		return nil, err
	}

	game, err := repo.Get(l.ctx, gameID)
	if err != nil {
		return nil, err
	}

	var envs []*ports.GameEnvDef
	envs, err = repo.ListEnvRecords(l.ctx, gameID)
	if err != nil && !errors.Is(err, svc.ErrGameNotFound) {
		return nil, err
	}

	info := gameToInfo(game, envs)

	return &types.GameDetailResponse{
		Code:    0,
		Message: "success",
		Data:    info,
	}, nil
}
