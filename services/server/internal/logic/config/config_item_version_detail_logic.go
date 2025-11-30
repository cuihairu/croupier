package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigItemVersionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigItemVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigItemVersionDetailLogic {
	return &ConfigItemVersionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigItemVersionDetailLogic) ConfigItemVersionDetail(id string, version int) (map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	if id == "" || version <= 0 {
		return nil, errors.New("无效的配置ID或版本号")
	}
	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, id, version)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":      record.Key,
		"version": record.Version,
		"format":  record.Format,
		"content": record.Value,
		"game_id": record.GameID,
		"env":     record.Env,
	}, nil
}
