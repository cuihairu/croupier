// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovalGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取审批详情
func NewApprovalGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalGetLogic {
	return &ApprovalGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalGetLogic) ApprovalGet(req *types.ApprovalGetRequest) (resp *types.ApprovalGetResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
