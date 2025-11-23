// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package support

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportTicketCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建工单
func NewSupportTicketCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportTicketCreateLogic {
	return &SupportTicketCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportTicketCreateLogic) SupportTicketCreate(req *types.SupportTicketCreateRequest) (resp *types.SupportTicketCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
