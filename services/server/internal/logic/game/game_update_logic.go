// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type GameUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新游戏
func NewGameUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameUpdateLogic {
	return &GameUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameUpdateLogic) GameUpdate(req *types.GameUpdateRequest) (*types.GameUpdateResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权更新游戏", "admin:all", "games:manage"); err != nil {
		return nil, err
	}

	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if v := strings.TrimSpace(req.Name); v != "" {
		name, err := sanitizeGameName(v)
		if err != nil {
			return nil, err
		}
		exists, err := l.svcCtx.GameModel.ExistsByNameIgnoreCase(l.ctx, name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errorx.NewConflict("game_id 已存在: " + name)
		}
		updates["name"] = name
	}
	if v := strings.TrimSpace(req.AliasName); v != "" {
		updates["alias_name"] = v
	}
	if v := strings.TrimSpace(req.Description); v != "" {
		updates["description"] = v
	}
	if v := strings.TrimSpace(req.Config); v != "" {
		updates["config"] = v
	}
	if v, err := sanitizeStatus(req.Status); err != nil {
		return nil, err
	} else if v != "" {
		updates["status"] = v
	}

	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}

	if err := l.svcCtx.GameModel.Update(l.ctx, id, updates); err != nil {
		return nil, err
	}

	l.svcCtx.InvalidateGameCache(l.ctx, id)

	game, err := l.svcCtx.GetGameCached(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.GameUpdateResponse{
		Code:    0,
		Message: "OK",
		Data:    buildGameInfo(game),
	}, nil
}
