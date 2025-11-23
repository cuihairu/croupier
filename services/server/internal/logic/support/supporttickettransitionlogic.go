// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketTransitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 工单状态转换
func NewSupportTicketTransitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketTransitionLogic {
	return &SupportTicketTransitionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketTransitionLogic) SupportTicketTransition(req *types.SupportTicketTransitionRequest) (resp *types.SupportTicketTransitionResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
