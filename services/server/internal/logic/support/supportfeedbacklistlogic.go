// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFeedbackListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取反馈列表
func NewSupportFeedbackListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackListLogic {
	return &SupportFeedbackListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackListLogic) SupportFeedbackList(req *types.SupportFeedbackListRequest) (resp *types.SupportFeedbackListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
