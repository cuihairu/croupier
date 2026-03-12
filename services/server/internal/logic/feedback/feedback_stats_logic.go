// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FeedbackStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取反馈统计
func NewFeedbackStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackStatsLogic {
	return &FeedbackStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackStatsLogic) FeedbackStats(req *types.FeedbackStatsRequest) (resp *types.FeedbackStatsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
