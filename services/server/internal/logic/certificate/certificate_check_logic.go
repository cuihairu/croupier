// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateCheckLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查证书状态
func NewCertificateCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckLogic {
	return &CertificateCheckLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateCheckLogic) CertificateCheck(req *types.CertificateCheckRequest) (resp *types.CertificateCheckResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
