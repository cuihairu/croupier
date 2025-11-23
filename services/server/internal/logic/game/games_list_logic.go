package game

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

// gameToInfo converts ports.Game to types.GameInfo
func gameToInfo(g *ports.Game, envs []*ports.GameEnvDef) types.GameInfo {
	info := types.GameInfo{
		ID:          g.ID,
		Name:        g.Name,
		Icon:        g.Icon,
		Description: g.Description,
		Enabled:     g.Enabled,
		AliasName:   g.AliasName,
		Homepage:    g.Homepage,
		Status:      g.Status,
		GameType:    g.GameType,
		GenreCode:   g.GenreCode,
		CreatedAt:   g.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   g.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Convert env definitions to env items
	if len(envs) > 0 {
		info.Envs = make([]types.GameEnvItem, 0, len(envs))
		for _, e := range envs {
			if e != nil {
				info.Envs = append(info.Envs, types.GameEnvItem{
					Env:         e.Env,
					Description: e.Description,
					Color:       e.Color,
				})
			}
		}
	}

	return info
}

func (l *GamesListLogic) GamesList(req *types.GamesListRequest) (*types.GamesListResponse, error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return &types.GamesListResponse{Code: 0, Message: "success", Data: types.GamesData{Games: []types.GameInfo{}}}, nil
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
	return &types.GamesListResponse{Code: 0, Message: "success", Data: types.GamesData{Games: out}}, nil
}
