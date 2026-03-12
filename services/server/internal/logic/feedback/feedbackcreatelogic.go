// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FeedbackCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建反馈
func NewFeedbackCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackCreateLogic {
	return &FeedbackCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackCreateLogic) FeedbackCreate(req *types.FeedbackCreateRequest) (resp *types.FeedbackDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
