// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"

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
	// todo: add your logic here and delete this line

	return
}
