package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigsListLogic {
	return &ConfigsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigsListLogic) ConfigsList(req *types.ConfigsListQuery) (*types.ConfigsListResponse, error) {
	filterGame := ""
	filterEnv := ""
	filterFormat := ""
	filterIDLike := ""
	if req != nil {
		filterGame = strings.TrimSpace(req.GameId)
		filterEnv = strings.TrimSpace(req.Env)
		filterFormat = strings.ToLower(strings.TrimSpace(req.Format))
		filterIDLike = strings.ToLower(strings.TrimSpace(req.IdLike))
	}
	items := make([]types.ConfigListItem, 0)
	for _, entry := range l.svcCtx.ConfigsSnapshot() {
		if entry == nil {
			continue
		}
		if filterGame != "" && !strings.EqualFold(entry.GameID, filterGame) {
			continue
		}
		if filterEnv != "" && !strings.EqualFold(entry.Env, filterEnv) {
			continue
		}
		if filterFormat != "" && !strings.EqualFold(entry.Format, filterFormat) {
			continue
		}
		if filterIDLike != "" && !strings.Contains(strings.ToLower(entry.ID), filterIDLike) {
			continue
		}
		items = append(items, types.ConfigListItem{
			Id:            entry.ID,
			GameId:        entry.GameID,
			Env:           entry.Env,
			Format:        entry.Format,
			LatestVersion: entry.Latest,
		})
	}
	return &types.ConfigsListResponse{Items: items}, nil
}
