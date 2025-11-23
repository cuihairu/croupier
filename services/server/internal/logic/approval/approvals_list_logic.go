// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovalsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取审批列表
func NewApprovalsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalsListLogic {
	return &ApprovalsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalsListLogic) ApprovalsList(req *types.ApprovalsListRequest) (resp *types.ApprovalsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
