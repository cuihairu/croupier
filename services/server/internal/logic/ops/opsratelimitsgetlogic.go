// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取限流规则
func NewOpsRateLimitsGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsGetLogic {
	return &OpsRateLimitsGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsGetLogic) OpsRateLimitsGet(req *types.OpsRateLimitsGetRequest) (resp *types.OpsRateLimitsGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
