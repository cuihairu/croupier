// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FeedbackDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除反馈
func NewFeedbackDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FeedbackDeleteLogic {
	return &FeedbackDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FeedbackDeleteLogic) FeedbackDelete(req *types.FeedbackDeleteRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
