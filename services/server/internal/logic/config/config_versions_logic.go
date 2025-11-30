// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取配置版本列表
func NewConfigVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionsLogic {
	return &ConfigVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionsLogic) ConfigVersions(req *types.ConfigVersionsRequest) (*types.ConfigVersionsResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("配置键不能为空")
	}

	versions, err := l.svcCtx.ConfigVersionModel.List(l.ctx, key)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(versions))
	for i := range versions {
		items = append(items, mapConfigVersion(&versions[i], true))
	}

	return &types.ConfigVersionsResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"key":      key,
			"total":    len(items),
			"versions": items,
		},
	}, nil
}
