// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateExpiringLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取即将过期的证书
func NewCertificateExpiringLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateExpiringLogic {
	return &CertificateExpiringLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateExpiringLogic) CertificateExpiring(req *types.CertificateExpiringRequest) (resp *types.CertificateExpiringResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
