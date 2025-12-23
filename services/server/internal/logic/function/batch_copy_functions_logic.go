// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchCopyFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量复制函数
func NewBatchCopyFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCopyFunctionsLogic {
	return &BatchCopyFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCopyFunctionsLogic) BatchCopyFunctions(req *types.BatchCopyFunctionsRequest) (*types.BatchCopyFunctionsResponse, error) {
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &types.BatchCopyFunctionsResponse{
			Updated: 0,
			Failed:  []string{"no function ids provided"},
			Copied:  []string{},
		}, nil
	}

	// 2. Call model layer to batch copy
	updated, failed, copied, err := l.svcCtx.FunctionModel.BatchCopyFunctions(l.ctx, req.FunctionIds)
	if err != nil {
		return nil, err
	}

	// 3. Return result
	return &types.BatchCopyFunctionsResponse{
		Updated: updated,
		Failed:  failed,
		Copied:  copied,
	}, nil
}
