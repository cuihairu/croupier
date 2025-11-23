// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFeedbackCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建反馈
func NewSupportFeedbackCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackCreateLogic {
	return &SupportFeedbackCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackCreateLogic) SupportFeedbackCreate(req *types.SupportFeedbackCreateRequest) (resp *types.SupportFeedbackCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
