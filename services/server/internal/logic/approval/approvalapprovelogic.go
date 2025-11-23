// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovalApproveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通过审批
func NewApprovalApproveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalApproveLogic {
	return &ApprovalApproveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalApproveLogic) ApprovalApprove(req *types.ApprovalApproveRequest) (resp *types.ApprovalApproveResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
