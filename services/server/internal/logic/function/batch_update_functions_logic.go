// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchUpdateFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量更新函数状态
func NewBatchUpdateFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchUpdateFunctionsLogic {
	return &BatchUpdateFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchUpdateFunctionsLogic) BatchUpdateFunctions(req *types.BatchUpdateFunctionsRequest) (*types.BatchUpdateFunctionsResponse, error) {
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &types.BatchUpdateFunctionsResponse{
			Updated: 0,
			Failed:  []string{"no function ids provided"},
		}, nil
	}

	// 2. Call model layer to batch update
	updated, failed, err := l.svcCtx.FunctionModel.BatchUpdateStatus(l.ctx, req.FunctionIds, req.Enabled)
	if err != nil {
		return nil, err
	}

	// 3. Return result
	return &types.BatchUpdateFunctionsResponse{
		Updated: updated,
		Failed:  failed,
	}, nil
}
