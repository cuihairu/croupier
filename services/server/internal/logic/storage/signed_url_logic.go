// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignedUrlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取签名URL
func NewSignedUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignedUrlLogic {
	return &SignedUrlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SignedUrlLogic) SignedUrl(req *types.SignedUrlRequest) (resp *types.SignedUrlResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
