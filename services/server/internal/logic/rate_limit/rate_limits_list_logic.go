// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取限流规则列表
func NewRateLimitsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitsListLogic {
	return &RateLimitsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitsListLogic) RateLimitsList(req *types.RateLimitsListRequest) (resp *types.RateLimitsListResponse, err error) {
	resource := strings.TrimSpace(req.Resource)
	limits, err := l.svcCtx.RateLimitModel.List(l.ctx, resource)
	if err != nil {
		return nil, err
	}

	resp = &types.RateLimitsListResponse{
		Items: make([]types.RateLimit, 0, len(limits)),
	}
	for i := range limits {
		resp.Items = append(resp.Items, buildRateLimitResponse(&limits[i]))
	}
	return resp, nil
}
