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

func (l *BatchCopyFunctionsLogic) BatchCopyFunctions(req *types.BatchCopyFunctionsRequest) (resp *types.BatchCopyFunctionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
