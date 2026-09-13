package function

import (
	"context"

	"github.com/cuihairu/croupier/internal/svc"
)

type BatchCopyFunctionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量复制函数
func NewBatchCopyFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCopyFunctionsLogic {
	return &BatchCopyFunctionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCopyFunctionsLogic) BatchCopyFunctions(req *BatchCopyFunctionsRequest) (*BatchCopyFunctionsResponse, error) {
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &BatchCopyFunctionsResponse{
			Updated: 0,
			Failed:  []string{},
			Copied:  []string{},
		}, nil
	}

	// 2. Call model layer to batch copy（单条失败进 failedIDs，无整体错误）
	updated, failed, copied := l.svcCtx.FunctionModel.BatchCopyFunctions(l.ctx, req.FunctionIds)

	// 3. Return result
	return &BatchCopyFunctionsResponse{
		Updated: updated,
		Failed:  failed,
		Copied:  copied,
	}, nil
}
