// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package approval

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ApprovalsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取审批列表
func NewApprovalsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalsListLogic {
	return &ApprovalsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalsListLogic) ApprovalsList(req *types.ApprovalsListRequest) (resp *types.ApprovalsListResponse, err error) {
	if l.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}

	filter := approvals.Filter{
		State: strings.TrimSpace(req.Status),
	}
	items, total, err := l.svcCtx.ApprovalsStore.List(filter, approvals.Page{
		Page: page,
		Size: size,
	})
	if err != nil {
		return nil, err
	}

	list := make([]approvalSummary, 0, len(items))
	for _, item := range items {
		list = append(list, buildApprovalSummary(item))
	}

	return &types.ApprovalsListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"approvals": list,
			"total":     total,
			"page":      page,
			"size":      size,
		},
	}, nil
}
