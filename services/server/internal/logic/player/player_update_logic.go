// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package player

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type PlayerUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新玩家信息
func NewPlayerUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerUpdateLogic {
	return &PlayerUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerUpdateLogic) PlayerUpdate(req *types.PlayerUpdateRequest) (*types.PlayerUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "玩家ID")
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Nickname); v != "" {
		updates["nickname"] = v
	}
	if v := strings.TrimSpace(req.Email); v != "" {
		updates["email"] = v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		updates["phone"] = v
	}
	if req.Status != 0 {
		if req.Status != model.PlayerStatusActive &&
			req.Status != model.PlayerStatusBanned &&
			req.Status != model.PlayerStatusSuspended {
			return nil, errors.New("状态值无效")
		}
		updates["status"] = req.Status
	}
	if req.Level > 0 {
		updates["level"] = req.Level
	}
	if req.Vip >= 0 {
		updates["vip"] = req.Vip
	}

	if len(updates) == 0 {
		return nil, errors.New("至少提供一个更新字段")
	}

	if err := l.svcCtx.PlayerModel.Update(l.ctx, id, updates); err != nil {
		return nil, err
	}

	player, err := l.svcCtx.PlayerModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.PlayerUpdateResponse{
		Player: utils.BuildPlayer(player),
	}, nil
}
