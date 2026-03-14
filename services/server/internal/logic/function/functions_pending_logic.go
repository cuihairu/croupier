// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type FunctionsPendingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取待处理函数
func NewFunctionsPendingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionsPendingLogic {
	return &FunctionsPendingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionsPendingLogic) FunctionsPending(req *types.FunctionsPendingRequest) (*types.FunctionsPendingResponse, error) {
	pending, err := l.svcCtx.FunctionModel.ListPending(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.PendingFunction, 0, len(pending))
	for _, p := range pending {
		items = append(items, types.PendingFunction{
			Id:        p.FunctionID,
			Name:      toString(p.Payload["name"]),
			Status:    p.Status,
			Requester: p.RequestedBy,
			CreatedAt: utils.FormatTimestamp(p.CreatedAt),
		})
	}

	return &types.FunctionsPendingResponse{
		Items: items,
	}, nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
