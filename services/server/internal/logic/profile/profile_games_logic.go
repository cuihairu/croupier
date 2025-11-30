// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfileGamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取我的游戏
func NewProfileGamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileGamesLogic {
	return &ProfileGamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProfileGamesLogic) ProfileGames(req *types.ProfileGamesRequest) (resp *types.ProfileGamesResponse, err error) {
	admin, _, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	if l.svcCtx.ProfileModel == nil {
		return nil, errors.New("ProfileModel 未初始化")
	}

	games, err := l.svcCtx.ProfileModel.ListGames(l.ctx, admin.ID)
	if err != nil {
		return nil, fmt.Errorf("获取游戏列表失败: %w", err)
	}

	respGames := make([]types.ProfileGame, 0, len(games))
	for i := range games {
		record := games[i]
		respGames = append(respGames, types.ProfileGame{
			GameId:      record.GameID,
			GameName:    record.GameName,
			Envs:        utils.DecodeStringSlice(record.Envs),
			Permissions: utils.DecodeStringSlice(record.Permissions),
		})
	}

	return &types.ProfileGamesResponse{
		Games: respGames,
	}, nil
}
