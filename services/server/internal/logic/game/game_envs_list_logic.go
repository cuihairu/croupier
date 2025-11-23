package game
import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameEnvsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameEnvsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvsListLogic {
	return &GameEnvsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvsListLogic) GameEnvsList(req *types.GameEnvsListRequest) (resp *types.GameEnvsListResponse, err error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return &types.GameEnvsListResponse{
			Code:    0,
			Message: "success",
			Data:    types.GameEnvsData{Envs: []types.GameEnvItem{}},
		}, nil
	}

	// Convert string ID to uint
	// TODO: Handle ID parsing properly
	var gameID uint = 0 // placeholder

	recs, err := repo.ListEnvRecords(l.ctx, gameID)
	if err != nil && !errors.Is(err, svc.ErrGameNotFound) {
		return nil, err
	}

	items := make([]types.GameEnvItem, 0, len(recs))
	for _, e := range recs {
		if e == nil {
			continue
		}
		items = append(items, types.GameEnvItem{
			Env:         e.Env,
			Description: e.Description,
			Color:       e.Color,
		})
	}

	return &types.GameEnvsListResponse{
		Code:    0,
		Message: "success",
		Data:    types.GameEnvsData{Envs: items},
	}, nil
}
