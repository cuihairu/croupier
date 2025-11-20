package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameDeleteLogic {
	return &GameDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameDeleteLogic) GameDelete(req *types.GameDetailRequest) error {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return svc.ErrGameNotFound
	}
	return repo.Delete(l.ctx, uint(req.Id))
}
