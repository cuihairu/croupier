package logic

import (
	"context"
	"errors"
	"strings"

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

func (l *GameEnvsListLogic) GameEnvsList(req *types.GameDetailRequest) (*types.GameEnvsResponse, error) {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return nil, svc.ErrGameNotFound
	}
	recs, err := repo.ListEnvRecords(l.ctx, uint(req.Id))
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
	return &types.GameEnvsResponse{Envs: items}, nil
}

type GameEnvAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameEnvAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvAddLogic {
	return &GameEnvAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvAddLogic) GameEnvAdd(req *types.GameEnvAddRequest) error {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return svc.ErrGameNotFound
	}
	env := strings.TrimSpace(req.Env)
	if env == "" {
		return ErrInvalidRequest
	}
	return repo.AddEnvWithMeta(l.ctx, uint(req.Id), env, strings.TrimSpace(req.Description), strings.TrimSpace(req.Color))
}

type GameEnvUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameEnvUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvUpdateLogic {
	return &GameEnvUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvUpdateLogic) GameEnvUpdate(req *types.GameEnvUpdateRequest) error {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return svc.ErrGameNotFound
	}
	if strings.TrimSpace(req.OldEnv) == "" {
		return ErrInvalidRequest
	}
	return repo.UpdateEnv(l.ctx, uint(req.Id), strings.TrimSpace(req.OldEnv), strings.TrimSpace(req.Env), strings.TrimSpace(req.Description), strings.TrimSpace(req.Color))
}

type GameEnvDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGameEnvDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameEnvDeleteLogic {
	return &GameEnvDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameEnvDeleteLogic) GameEnvDelete(req *types.GameEnvDeleteRequest) error {
	repo := l.svcCtx.GamesRepository()
	if repo == nil {
		return svc.ErrGameNotFound
	}
	if strings.TrimSpace(req.Env) == "" {
		return ErrInvalidRequest
	}
	return repo.RemoveEnv(l.ctx, uint(req.Id), strings.TrimSpace(req.Env))
}
