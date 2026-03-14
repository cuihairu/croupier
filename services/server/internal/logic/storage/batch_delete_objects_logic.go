// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type BatchDeleteObjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除对象
func NewBatchDeleteObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteObjectsLogic {
	return &BatchDeleteObjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteObjectsLogic) BatchDeleteObjects(req *types.BatchDeleteObjectsRequest) (resp *types.BatchDeleteObjectsResponse, err error) {
	return nil, errorx.NewNotImplemented("BatchDeleteObjects not implemented")
}
