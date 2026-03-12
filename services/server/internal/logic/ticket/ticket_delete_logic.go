// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除工单
func NewTicketDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketDeleteLogic {
	return &TicketDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketDeleteLogic) TicketDelete(req *types.TicketDeleteRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
