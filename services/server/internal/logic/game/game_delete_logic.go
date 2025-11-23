package game
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

func (l *GameDeleteLogic) GameDelete(req *types.GameDeleteRequest) (resp *types.GameDeleteResponse, err error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return nil, svc.ErrGameNotFound
	}

	gameID, err := parseID(req.ID)
	if err != nil {
		return nil, err
	}

	if err := repo.Delete(l.ctx, gameID); err != nil {
		return nil, err
	}

	return &types.GameDeleteResponse{
		Code:    0,
		Message: "success",
	}, nil
}
