// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ApprovalGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取审批详情
func NewApprovalGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalGetLogic {
	return &ApprovalGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalGetLogic) ApprovalGet(req *types.ApprovalGetRequest) (resp *types.ApprovalGetResponse, err error) {
	if l.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}

	approval, err := l.svcCtx.ApprovalsStore.Get(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}

	detail := buildApprovalDetail(approval)

	return &types.ApprovalGetResponse{
		Code:    0,
		Message: "OK",
		Data:    detail,
	}, nil
}
