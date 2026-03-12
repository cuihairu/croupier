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

func (l *BatchUpdateFunctionsLogic) BatchUpdateFunctions(req *types.BatchUpdateFunctionsRequest) (resp *types.BatchUpdateFunctionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
