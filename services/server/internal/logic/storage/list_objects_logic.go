// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListObjectsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出对象
func NewListObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListObjectsLogic {
	return &ListObjectsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListObjectsLogic) ListObjects(req *types.ListObjectsRequest) (resp *types.ListObjectsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
