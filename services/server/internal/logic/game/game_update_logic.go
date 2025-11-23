package game
import (
	"context"
	"errors"
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

func (l *GameUpdateLogic) GameUpdate(req *types.GameUpdateRequest) (resp *types.GameUpdateResponse, err error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}

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

	applyGameUpdates(game, req)

	if err := repo.Update(l.ctx, game); err != nil {
		return nil, err
	}

	// 获取更新后的游戏信息（包括环境列表）
	var envs []*ports.GameEnvDef
	envs, err = repo.ListEnvRecords(l.ctx, gameID)
	if err != nil && !errors.Is(err, svc.ErrGameNotFound) {
		return nil, err
	}

	info := gameToInfo(game, envs)

	return &types.GameUpdateResponse{
		Code:    0,
		Message: "success",
		Data:    info,
	}, nil
}

func applyGameUpdates(game *ports.Game, req *types.GameUpdateRequest) {
	if strings.TrimSpace(req.Name) != "" {
		game.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Description) != "" {
		game.Description = strings.TrimSpace(req.Description)
	}
	if strings.TrimSpace(req.Config) != "" {
		// TODO: Handle config field - may need to parse and validate
		// For now we skip it as ports.Game may not have a Config field
	}
	if strings.TrimSpace(req.Status) != "" {
		game.Status = strings.TrimSpace(req.Status)
	}
}
