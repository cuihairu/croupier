// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type SignedUrlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取签名URL
func NewSignedUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignedUrlLogic {
	return &SignedUrlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SignedUrlLogic) SignedUrl(req *types.SignedUrlRequest) (resp *types.SignedUrlResponse, err error) {
	return nil, errorx.NewNotImplemented("SignedUrl not implemented")
}
