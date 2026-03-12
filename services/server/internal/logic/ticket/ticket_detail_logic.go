// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单详情
func NewTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketDetailLogic {
	return &TicketDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketDetailLogic) TicketDetail(req *types.TicketDetailRequest) (resp *types.TicketDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
