
package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
)

type BatchDeleteFunctionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除函数
func NewBatchDeleteFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteFunctionsLogic {
	return &BatchDeleteFunctionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteFunctionsLogic) BatchDeleteFunctions(req *BatchDeleteFunctionsRequest) (*BatchDeleteFunctionsResponse, error) {
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &BatchDeleteFunctionsResponse{
			Updated: 0,
			Failed:  []string{"no function ids provided"},
		}, nil
	}

	// 2. Call model layer to batch delete
	updated, failed, err := l.svcCtx.FunctionModel.BatchDeleteFunctions(l.ctx, req.FunctionIds)
	if err != nil {
		return nil, err
	}

	// 3. Return result
	return &BatchDeleteFunctionsResponse{
		Updated: updated,
		Failed:  failed,
	}, nil
}
