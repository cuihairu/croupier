// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type ConfigUpsertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建或更新配置
func NewConfigUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigUpsertLogic {
	return &ConfigUpsertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigUpsertLogic) ConfigUpsert(req *types.ConfigUpsertRequest) (*types.ConfigUpsertResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("配置键不能为空")
	}

	record, err := l.svcCtx.ConfigVersionModel.Create(l.ctx, key, req.Value, configAuthor(l.ctx))
	if err != nil {
		return nil, err
	}

	return &types.ConfigUpsertResponse{
		Code:    0,
		Message: "OK",
		Data:    mapConfigVersion(record, true),
	}, nil
}
