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

type ApprovalRejectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 拒绝审批
func NewApprovalRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalRejectLogic {
	return &ApprovalRejectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalRejectLogic) ApprovalReject(req *types.ApprovalRejectRequest) (resp *types.ApprovalRejectResponse, err error) {
	if l.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, errors.New("reason 不能为空")
	}

	record, err := l.svcCtx.ApprovalsStore.Reject(strings.TrimSpace(req.ID), strings.TrimSpace(req.Reason))
	if err != nil {
		return nil, err
	}

	return &types.ApprovalRejectResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":     record.ID,
			"state":  record.State,
			"reason": record.Reason,
		},
	}, nil
}
