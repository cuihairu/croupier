package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *FunctionsPendingLogic) FunctionsPending(req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	pending, err := l.svcCtx.FunctionModel.ListPending(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]PendingFunction, 0, len(pending))
	for _, p := range pending {
		items = append(items, PendingFunction{
			ID:        p.FunctionID,
			Name:      toString(p.Payload["name"]),
			Status:    p.Status,
			Requester: p.RequestedBy,
			CreatedAt: utils.FormatTimestamp(p.CreatedAt),
		})
	}

	return &FunctionsPendingResponse{
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
