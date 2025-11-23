// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工单详情
func NewSupportTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketDetailLogic {
	return &SupportTicketDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketDetailLogic) SupportTicketDetail(req *types.SupportTicketDetailRequest) (resp *types.SupportTicketDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
