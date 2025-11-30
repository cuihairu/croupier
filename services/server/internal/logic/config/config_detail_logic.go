package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigDetailLogic {
	return &ConfigDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigDetailLogic) ConfigDetail(id string) (map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("config id required")
	}
	record, err := l.svcCtx.ConfigVersionModel.FindLatest(l.ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":      record.Key,
		"format":  record.Format,
		"content": record.Value,
		"version": record.Version,
		"game_id": record.GameID,
		"env":     record.Env,
	}, nil
}
