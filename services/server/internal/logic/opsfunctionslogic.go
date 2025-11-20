package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsFunctionsLogic {
	return &OpsFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsFunctionsLogic) OpsFunctions() (*types.OpsFunctionsResponse, error) {
	descs := l.svcCtx.DescriptorsSnapshot()
	items := make([]types.OpsFunction, 0, len(descs))
	for _, d := range descs {
		if d == nil || d.ID == "" {
			continue
		}
		items = append(items, types.OpsFunction{
			Id:       d.ID,
			Category: d.Category,
		})
	}
	return &types.OpsFunctionsResponse{Functions: items}, nil
}
