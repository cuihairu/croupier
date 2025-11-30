// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package game

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GameUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新游戏
func NewGameUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GameUpdateLogic {
	return &GameUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GameUpdateLogic) GameUpdate(req *types.GameUpdateRequest) (*types.GameUpdateResponse, error) {
	id, err := parseGameID(req.ID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if v := strings.TrimSpace(req.Name); v != "" {
		updates["name"] = v
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
		return nil, fmt.Errorf("请提供需要更新的字段")
	}

	if err := l.svcCtx.GameModel.Update(l.ctx, id, updates); err != nil {
		return nil, err
	}

	game, err := l.svcCtx.GameModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.GameUpdateResponse{
		Code:    0,
		Message: "OK",
		Data:    buildGameInfo(game),
	}, nil
}
