package config

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type ConfigsListRequest struct {
	GameID string `form:"game_id,optional"`
	Env    string `form:"env,optional"`
	Format string `form:"format,optional"`
	IDLike string `form:"id_like,optional"`
}

func NewConfigsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigsListLogic {
	return &ConfigsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigsListLogic) ConfigsList(req *ConfigsListRequest) (map[string]interface{}, error) {
	opts := model.ConfigListOptions{
		GameID: req.GameID,
		Env:    req.Env,
		Format: req.Format,
		IDLike: req.IDLike,
	}
	records, err := l.svcCtx.ConfigVersionModel.ListLatest(l.ctx, opts)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		items = append(items, mapConfigItem(&records[i]))
	}
	return map[string]interface{}{
		"items": items,
	}, nil
}
