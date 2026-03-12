// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单列表
func NewTicketsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketsListLogic {
	return &TicketsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketsListLogic) TicketsList(req *types.TicketsListRequest) (resp *types.TicketsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
