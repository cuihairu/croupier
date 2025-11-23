// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除限流规则
func NewOpsRateLimitsDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsDeleteLogic {
	return &OpsRateLimitsDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsDeleteLogic) OpsRateLimitsDelete(req *types.RateLimitDeleteRequest) (resp *types.OpsRateLimitsDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
