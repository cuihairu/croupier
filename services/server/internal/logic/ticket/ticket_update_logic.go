// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新工单
func NewTicketUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketUpdateLogic {
	return &TicketUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketUpdateLogic) TicketUpdate(req *types.TicketUpdateRequest) (resp *types.TicketDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
