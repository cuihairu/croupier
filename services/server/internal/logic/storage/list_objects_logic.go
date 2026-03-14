// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type ListObjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出对象
func NewListObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListObjectsLogic {
	return &ListObjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListObjectsLogic) ListObjects(req *types.ListObjectsRequest) (resp *types.ListObjectsResponse, err error) {
	return nil, errorx.NewNotImplemented("ListObjects not implemented")
}
