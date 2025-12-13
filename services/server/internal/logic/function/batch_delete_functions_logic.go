// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDeleteFunctionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除函数
func NewBatchDeleteFunctionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteFunctionsLogic {
	return &BatchDeleteFunctionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteFunctionsLogic) BatchDeleteFunctions(req *types.BatchDeleteFunctionsRequest) (*types.BatchDeleteFunctionsResponse, error) {
	// TODO: implement batch delete logic
	// 1. Validate request
	if len(req.FunctionIds) == 0 {
		return &types.BatchDeleteFunctionsResponse{
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
	return &types.BatchDeleteFunctionsResponse{
		Updated: updated,
		Failed:  failed,
	}, nil
}