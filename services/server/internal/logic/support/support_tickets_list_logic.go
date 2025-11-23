// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单列表
func NewSupportTicketsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketsListLogic {
	return &SupportTicketsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketsListLogic) SupportTicketsList(req *types.SupportTicketsListRequest) (resp *types.SupportTicketsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
