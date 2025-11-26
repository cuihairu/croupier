// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovalRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 拒绝审批
func NewApprovalRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalRejectLogic {
	return &ApprovalRejectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalRejectLogic) ApprovalReject(req *types.ApprovalRejectRequest) (resp *types.ApprovalRejectResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
