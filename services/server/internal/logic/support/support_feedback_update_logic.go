// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFeedbackUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新反馈
func NewSupportFeedbackUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackUpdateLogic {
	return &SupportFeedbackUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackUpdateLogic) SupportFeedbackUpdate(req *types.SupportFeedbackUpdateRequest) (resp *types.SupportFeedbackUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
