// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketTransitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 工单状态转换
func NewTicketTransitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketTransitionLogic {
	return &TicketTransitionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketTransitionLogic) TicketTransition(req *types.TicketTransitionRequest) (resp *types.TicketDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
