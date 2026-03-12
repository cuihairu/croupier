// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateDomainInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取域名证书信息
func NewCertificateDomainInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDomainInfoLogic {
	return &CertificateDomainInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateDomainInfoLogic) CertificateDomainInfo(req *types.CertificateDomainInfoRequest) (resp *types.CertificateDomainInfoResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
