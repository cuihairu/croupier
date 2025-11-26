// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加证书
func NewCertificateAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAddLogic {
	return &CertificateAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAddLogic) CertificateAdd(req *types.CertificateAddRequest) (resp *types.CertificateAddResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
