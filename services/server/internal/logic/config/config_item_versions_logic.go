package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigItemVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfigItemVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigItemVersionsLogic {
	return &ConfigItemVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigItemVersionsLogic) ConfigItemVersions(id string) (map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("配置ID不能为空")
	}
	versions, err := l.svcCtx.ConfigVersionModel.List(l.ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(versions))
	for i := range versions {
		items = append(items, mapConfigVersion(&versions[i], true))
	}
	return map[string]interface{}{
		"versions": items,
	}, nil
}
