// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadObjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 上传对象
func NewUploadObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadObjectLogic {
	return &UploadObjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadObjectLogic) UploadObject(req *types.UploadObjectRequest) (resp *types.UploadObjectResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
