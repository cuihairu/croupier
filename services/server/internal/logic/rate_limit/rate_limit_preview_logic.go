// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rate_limit

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RateLimitPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 预览限流规则
func NewRateLimitPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RateLimitPreviewLogic {
	return &RateLimitPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RateLimitPreviewLogic) RateLimitPreview(req *types.RateLimitPreviewRequest) (resp *types.RateLimitPreviewResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
