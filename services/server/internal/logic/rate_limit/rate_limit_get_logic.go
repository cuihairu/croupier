// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取限流规则
func NewRateLimitGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitGetLogic {
	return &RateLimitGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitGetLogic) RateLimitGet(req *types.RateLimitGetRequest) (resp *types.RateLimitResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
