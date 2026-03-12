// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitUpsertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建/更新限流规则
func NewRateLimitUpsertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitUpsertLogic {
	return &RateLimitUpsertLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitUpsertLogic) RateLimitUpsert(req *types.RateLimitUpsertRequest) (resp *types.RateLimitResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
