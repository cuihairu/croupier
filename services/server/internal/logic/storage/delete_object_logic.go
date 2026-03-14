// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type DeleteObjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除对象
func NewDeleteObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteObjectLogic {
	return &DeleteObjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteObjectLogic) DeleteObject(req *types.DeleteObjectRequest) (resp *types.DeleteObjectResponse, err error) {
	return nil, errorx.NewNotImplemented("DeleteObject not implemented")
}
