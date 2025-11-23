// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFeedbackDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除反馈
func NewSupportFeedbackDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackDeleteLogic {
	return &SupportFeedbackDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackDeleteLogic) SupportFeedbackDelete(req *types.SupportFeedbackDeleteRequest) (resp *types.SupportFeedbackDeleteResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
