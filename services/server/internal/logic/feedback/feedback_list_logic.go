// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FeedbackListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取反馈列表
func NewFeedbackListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackListLogic {
	return &FeedbackListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackListLogic) FeedbackList(req *types.FeedbackListRequest) (resp *types.FeedbackListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
