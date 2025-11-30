// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"
	"errors"
	"strings"

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
	if l.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}

	record, err := l.svcCtx.ApprovalsStore.Approve(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}

	return &types.ApprovalApproveResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":    record.ID,
			"state": record.State,
		},
	}, nil
}
