// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ticket

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建工单
func NewTicketCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketCreateLogic {
	return &TicketCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TicketCreateLogic) TicketCreate(req *types.TicketCreateRequest) (resp *types.TicketDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
