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

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayerCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建玩家
func NewPlayerCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayerCreateLogic {
	return &PlayerCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PlayerCreateLogic) PlayerCreate(req *types.PlayerCreateRequest) (*types.PlayerCreateResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		return nil, errors.New("密码不能为空")
	}
	gameID := strings.TrimSpace(req.GameId)
	if gameID == "" {
		return nil, errors.New("Game ID 不能为空")
	}

	player := &model.Player{
		Username: username,
		Nickname: strings.TrimSpace(req.Nickname),
		Email:    strings.TrimSpace(req.Email),
		Phone:    strings.TrimSpace(req.Phone),
		GameID:   gameID,
		Status:   model.PlayerStatusActive,
	}

	if err := l.svcCtx.PlayerModel.Create(l.ctx, player, password); err != nil {
		return nil, err
	}

	return &types.PlayerCreateResponse{
		Player: utils.BuildPlayer(player),
	}, nil
}
