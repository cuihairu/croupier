package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
)

type BatchUpdateFunctionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量更新函数状态
func NewBatchUpdateFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchUpdateFunctionsLogic {
	return &BatchUpdateFunctionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchUpdateFunctionsLogic) BatchUpdateFunctions(req *BatchUpdateFunctionsRequest) (*BatchUpdateFunctionsResponse, error) {
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &BatchUpdateFunctionsResponse{
			Updated: 0,
			Failed:  []string{},
		}, nil
	}

	// 2. Call model layer to batch update
	updated, failed, err := l.svcCtx.FunctionModel.BatchUpdateStatus(l.ctx, req.FunctionIds, req.Enabled)
	if err != nil {
		return nil, err
	}

	// 3. Return result
	return &BatchUpdateFunctionsResponse{
		Updated: updated,
		Failed:  failed,
	}, nil
}
