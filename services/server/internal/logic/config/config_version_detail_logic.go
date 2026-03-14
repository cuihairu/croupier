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

type ConfigVersionDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取配置版本详情
func NewConfigVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigVersionDetailLogic {
	return &ConfigVersionDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigVersionDetailLogic) ConfigVersionDetail(req *types.ConfigVersionDetailRequest) (*types.ConfigVersionDetailResponse, error) {
	if req == nil {
		return nil, errors.New("请求参数不能为空")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("配置键不能为空")
	}
	if req.Version <= 0 {
		return nil, errors.New("版本号必须大于0")
	}

	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, key, req.Version)
	if err != nil {
		return nil, err
	}

	return &types.ConfigVersionDetailResponse{
		Code:    0,
		Message: "OK",
		Data:    mapConfigVersion(record, true),
	}, nil
}
