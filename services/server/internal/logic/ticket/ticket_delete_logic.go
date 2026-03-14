// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type TicketDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除工单
func NewTicketDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketDeleteLogic {
	return &TicketDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketDeleteLogic) TicketDelete(req *types.TicketDeleteRequest) error {
	id, err := parseTicketID(req.ID)
	if err != nil {
		return err
	}
	return l.svcCtx.TicketModel.Delete(l.ctx, id)
}
