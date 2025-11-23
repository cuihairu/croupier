// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsRateLimitsPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览限流规则
func NewOpsRateLimitsPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsRateLimitsPreviewLogic {
	return &OpsRateLimitsPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsRateLimitsPreviewLogic) OpsRateLimitsPreview(req *types.RateLimitPreviewQuery) (resp *types.OpsRateLimitsPreviewResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
