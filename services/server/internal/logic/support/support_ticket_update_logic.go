// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新工单
func NewSupportTicketUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketUpdateLogic {
	return &SupportTicketUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketUpdateLogic) SupportTicketUpdate(req *types.SupportTicketUpdateRequest) (resp *types.SupportTicketUpdateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
