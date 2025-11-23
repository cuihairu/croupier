// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新限流规则
func NewOpsRateLimitsUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsUpdateLogic {
	return &OpsRateLimitsUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsUpdateLogic) OpsRateLimitsUpdate(req *types.RateLimitRulesRequest) (resp *types.OpsRateLimitsUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
