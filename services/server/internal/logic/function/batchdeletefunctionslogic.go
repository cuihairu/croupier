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

func (l *BatchDeleteFunctionsLogic) BatchDeleteFunctions(req *types.BatchDeleteFunctionsRequest) (resp *types.BatchDeleteFunctionsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
